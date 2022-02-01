Run the application

`go run main.go sign --artifact LICENSE -c client.pem -s client.sig`

Exract the signing certicate

`openssl x509 -pubkey -noout -in cert.pem`

Extract the public key

`openssl x509 -pubkey -noout -in cert.pem > public.pem`

verify the signature

`openssl dgst -sha256 -verify public.pem -signature sig.bin file.txt`
