//
// Copyright 2021 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aws

import (
	"bytes"
	"context"
	"crypto"
	"io"

	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/pkg/errors"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/options"
)

var awsSupportedAlgorithms []string = []string{
	kms.CustomerMasterKeySpecRsa2048,
	kms.CustomerMasterKeySpecRsa3072,
	kms.CustomerMasterKeySpecRsa4096,
	kms.CustomerMasterKeySpecEccNistP256,
	kms.CustomerMasterKeySpecEccNistP384,
	kms.CustomerMasterKeySpecEccNistP521,
}

var awsSupportedHashFuncs = []crypto.Hash{
	crypto.SHA256,
	crypto.SHA384,
	crypto.SHA512,
}

type SignerVerifier struct {
	client *awsClient
}

// LoadSignerVerifier generates signatures using the specified key object in AWS KMS and hash algorithm.
//
// It also can verify signatures locally using the public key. hashFunc must not be crypto.Hash(0).
func LoadSignerVerifier(referenceStr string) (*SignerVerifier, error) {
	a := &SignerVerifier{}

	var err error
	a.client, err = newAWSClient(referenceStr)
	if err != nil {
		return nil, err
	}

	return a, nil
}

// THIS WILL BE REMOVED ONCE ALL SIGSTORE PROJECTS NO LONGER USE IT
func (a *SignerVerifier) Sign(ctx context.Context, payload []byte) ([]byte, []byte, error) {
	sig, err := a.SignMessage(bytes.NewReader(payload), options.WithContext(ctx))
	return sig, nil, err
}

// SignMessage signs the provided message using AWS KMS. If the message is provided,
// this method will compute the digest according to the hash function specified
// when the Signer was created.
//
// SignMessage recognizes the following Options listed in order of preference:
//
// - WithContext()
//
// - WithDigest()
//
// - WithCryptoSignerOpts()
//
// All other options are ignored if specified.
func (a *SignerVerifier) SignMessage(message io.Reader, opts ...signature.SignOption) ([]byte, error) {
	var digest []byte
	var err error
	ctx := context.Background()

	for _, opt := range opts {
		opt.ApplyContext(&ctx)
		opt.ApplyDigest(&digest)
	}

	var signerOpts crypto.SignerOpts
	signerOpts, err = a.client.getHashFunc(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "getting fetching default hash function")
	}
	for _, opt := range opts {
		opt.ApplyCryptoSignerOpts(&signerOpts)
	}

	hf := signerOpts.HashFunc()

	if len(digest) == 0 {
		digest, hf, err = signature.ComputeDigestForSigning(message, hf, awsSupportedHashFuncs, opts...)
		if err != nil {
			return nil, err
		}
	}

	return a.client.sign(ctx, digest, hf)
}

// PublicKey returns the public key that can be used to verify signatures created by
// this signer. If the caller wishes to specify the context to use to obtain
// the public key, pass option.WithContext(desiredCtx).
//
// All other options are ignored if specified.
func (a *SignerVerifier) PublicKey(opts ...signature.PublicKeyOption) (crypto.PublicKey, error) {
	ctx := context.Background()
	for _, opt := range opts {
		opt.ApplyContext(&ctx)
	}

	return a.client.public(ctx)
}

// VerifySignature verifies the signature for the given message. Unless provided
// in an option, the digest of the message will be computed using the hash function specified
// when the SignerVerifier was created.
//
// This function returns nil if the verification succeeded, and an error message otherwise.
//
// This function recognizes the following Options listed in order of preference:
//
// - WithContext()
//
// - WithDigest()
//
// - WithRemoteVerification()
//
// - WithCryptoSignerOpts()
//
// All other options are ignored if specified.
func (a *SignerVerifier) VerifySignature(sig, message io.Reader, opts ...signature.VerifyOption) (err error) {
	ctx := context.Background()
	var digest []byte
	var remoteVerification bool

	for _, opt := range opts {
		opt.ApplyContext(&ctx)
		opt.ApplyDigest(&digest)
		opt.ApplyRemoteVerification(&remoteVerification)
	}

	if !remoteVerification {
		return a.client.verify(ctx, sig, message, opts...)
	}

	var signerOpts crypto.SignerOpts
	signerOpts, err = a.client.getHashFunc(ctx)
	if err != nil {
		return errors.Wrap(err, "getting hash func")
	}
	for _, opt := range opts {
		opt.ApplyCryptoSignerOpts(&signerOpts)
	}
	hf := signerOpts.HashFunc()

	if len(digest) == 0 {
		digest, _, err = signature.ComputeDigestForVerifying(message, hf, awsSupportedHashFuncs, opts...)
		if err != nil {
			return err
		}
	}

	sigBytes, err := io.ReadAll(sig)
	if err != nil {
		return errors.Wrap(err, "reading signature")
	}
	return a.client.verifyRemotely(ctx, sigBytes, digest)
}

// CreateKey attempts to create a new key in Vault with the specified algorithm.
func (a *SignerVerifier) CreateKey(ctx context.Context, algorithm string) (crypto.PublicKey, error) {
	return a.client.createKey(ctx, algorithm)
}

type cryptoSignerWrapper struct {
	ctx      context.Context
	hashFunc crypto.Hash
	sv       *SignerVerifier
	errFunc  func(error)
}

func (c cryptoSignerWrapper) Public() crypto.PublicKey {
	pk, err := c.sv.PublicKey(options.WithContext(c.ctx))
	if err != nil && c.errFunc != nil {
		c.errFunc(err)
	}
	return pk
}

func (c cryptoSignerWrapper) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	hashFunc := c.hashFunc
	if opts != nil {
		hashFunc = opts.HashFunc()
	}
	awsOptions := []signature.SignOption{
		options.WithContext(c.ctx),
		options.WithDigest(digest),
		options.WithCryptoSignerOpts(hashFunc),
	}

	return c.sv.SignMessage(nil, awsOptions...)
}

func (a *SignerVerifier) CryptoSigner(ctx context.Context, errFunc func(error)) (crypto.Signer, crypto.SignerOpts, error) {
	defaultHf, err := a.client.getHashFunc(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "getting fetching default hash function")
	}

	csw := &cryptoSignerWrapper{
		ctx:      ctx,
		sv:       a,
		hashFunc: defaultHf,
		errFunc:  errFunc,
	}

	return csw, defaultHf, nil
}

func (*SignerVerifier) SupportedAlgorithms() []string {
	return awsSupportedAlgorithms
}

func (*SignerVerifier) DefaultAlgorithm() string {
	return kms.CustomerMasterKeySpecEccNistP256
}
