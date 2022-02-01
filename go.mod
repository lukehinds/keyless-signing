module github.com/lukehinds/keyless-sigstore

go 1.16

require (
	cloud.google.com/go/kms v1.1.0
	github.com/ReneKroon/ttlcache/v2 v2.11.0
	github.com/aws/aws-sdk-go v1.42.44
	github.com/coreos/go-oidc/v3 v3.1.0
	github.com/gabriel-vasile/mimetype v1.4.0
	github.com/go-openapi/errors v0.20.2
	github.com/go-openapi/runtime v0.22.0
	github.com/go-openapi/strfmt v0.21.1
	github.com/go-openapi/swag v0.21.1
	github.com/go-openapi/validate v0.20.3
	github.com/go-test/deep v1.0.8
	github.com/google/go-containerregistry v0.8.0
	github.com/hashicorp/vault/api v1.3.1
	github.com/pkg/errors v0.9.1
	github.com/segmentio/ksuid v1.0.4
	github.com/sigstore/rekor v0.4.0
	github.com/sigstore/sigstore v1.1.0
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966
	github.com/spf13/cobra v1.3.0
	github.com/spf13/viper v1.10.1
	github.com/theupdateframework/go-tuf v0.0.0-20220127213825-87caa18db2a6
	golang.org/x/crypto v0.0.0-20220131195533-30dcbda58838
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	google.golang.org/genproto v0.0.0-20220126215142-9970aeb2e350
	google.golang.org/protobuf v1.27.1
	gopkg.in/square/go-jose.v2 v2.6.0
)
