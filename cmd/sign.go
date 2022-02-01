/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/lukehinds/keyless-sigstore/pkg/cryptoutils"
	"github.com/lukehinds/keyless-sigstore/pkg/generated/client/operations"
	"github.com/lukehinds/keyless-sigstore/pkg/httpclients"
	"github.com/lukehinds/keyless-sigstore/pkg/oauthflow"
	"github.com/lukehinds/keyless-sigstore/pkg/signature"

	"github.com/lukehinds/keyless-sigstore/pkg/tlog"
	"github.com/lukehinds/keyless-sigstore/pkg/utils"
)

// signCmd represents the sign command
var signCmd = &cobra.Command{
	Use:   "sign",
	Short: "A brief description of your command",
	Long:  `A longer description that spans multiple lines.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		payload, err := ioutil.ReadFile(viper.GetString("artifact"))
		if err != nil {
			return err
		}

		mimetype, err := utils.GetFileType(viper.GetString("artifact"))
		if err != nil {
			return err
		}

		result := utils.FindString(mimetype)
		if !result {
			fmt.Println("File type currently not supported: ", mimetype)
			os.Exit(1)
		}

		// Retrieve idToken from oidc provider
		idToken, err := oauthflow.OIDConnect(
			viper.GetString("oidc-issuer"),
			viper.GetString("oidc-client-id"),
			viper.GetString("oidc-client-secret"),
			oauthflow.DefaultIDTokenGetter,
		)
		if err != nil {
			return err
		}
		fmt.Println("\nReceived OpenID Scope retrieved for account:", idToken.Subject)

		signer, _, err := signature.NewDefaultECDSASignerVerifier()
		if err != nil {
			return err
		}

		pub, err := signer.PublicKey()
		if err != nil {
			return err
		}
		pubBytes, err := cryptoutils.MarshalPublicKeyToDER(pub)
		if err != nil {
			return err
		}

		proof, err := signer.SignMessage(strings.NewReader(idToken.Subject))
		if err != nil {
			return err
		}

		certResp, err := httpclients.GetCert(idToken, proof, pubBytes, viper.GetString("fulcio-server"))
		if err != nil {
			switch t := err.(type) {
			case *operations.SigningCertDefault:
				if t.Code() == http.StatusInternalServerError {
					return err
				}
			default:
				return err
			}
			os.Exit(1)
		}

		certs, err := cryptoutils.UnmarshalCertificatesFromPEM([]byte(certResp.Payload))
		if err != nil {
			return err
		} else if len(certs) == 0 {
			return errors.New("no certificates were found in response")
		}
		signingCert := certs[0]
		signingCertPEM, err := cryptoutils.MarshalCertificateToPEM(signingCert)
		if err != nil {
			return err
		}

		fmt.Println("Received signing certificate with serial number: ", signingCert.SerialNumber)

		signature, err := signer.SignMessage(bytes.NewReader(payload))
		if err != nil {
			panic(fmt.Sprintf("Error occurred while during artifact signing: %s", err))
		}

		// Send to rekor
		fmt.Println("Sending entry to transparency log")
		tlogEntry, err := tlog.UploadToRekor(
			signingCertPEM,
			signature,
			viper.GetString("rekor-server"),
			payload,
		)
		if err != nil {
			return err
		}
		fmt.Printf("Rekor entry successful. URL: %v%v\n", viper.GetString("rekor-server"), tlogEntry)

		// If cert-out path is passed, save the signing certificate to file
		if viper.IsSet("cert-out") {
			err = ioutil.WriteFile(viper.GetString("cert-out"), signingCertPEM, 0600)
			if err != nil {
				return err
			}
		}

		// If sig-out path is passed, save the signature to file
		if viper.IsSet("sig-out") {
			err = ioutil.WriteFile(viper.GetString("sig-out"), signature, 0600)
			if err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(signCmd)
	signCmd.PersistentFlags().String("oidc-issuer", "https://oauth2.sigstore.dev/auth", "OIDC provider to be used to issue ID token")
	signCmd.PersistentFlags().String("oidc-client-id", "sigstore", "client ID for application")
	signCmd.PersistentFlags().String("oidc-client-secret", "", "client secret for application")
	signCmd.PersistentFlags().StringP("cert-out", "c", "-", "output file to write signing certificate")
	signCmd.PersistentFlags().StringP("sig-out", "s", "-", "output file to write signature")
	signCmd.PersistentFlags().StringP("artifact", "a", "", "artifact to sign")
	if err := viper.BindPFlags(signCmd.PersistentFlags()); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
