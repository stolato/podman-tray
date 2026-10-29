#!/bin/sh
# Creates a self-signed code signing identity in the login keychain. Signing every build
# with it (instead of ad-hoc) keeps the app's identity stable, so macOS doesn't treat each
# `make install` as a new app (login item, Terminal automation permission).
# It's only good for this Mac: distribution still needs a Developer ID.
set -eu

NAME="${1:-Podman Tray Local}"
KEYCHAIN="$HOME/Library/Keychains/login.keychain-db"

if security find-certificate -c "$NAME" "$KEYCHAIN" >/dev/null 2>&1; then
	echo "Certificado \"$NAME\" já existe."
	exit 0
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

cat >"$tmp/cert.cnf" <<EOF
[req]
distinguished_name = dn
x509_extensions = ext
prompt = no
[dn]
CN = $NAME
[ext]
basicConstraints = critical,CA:false
keyUsage = critical,digitalSignature
extendedKeyUsage = critical,codeSigning
EOF

# /usr/bin/openssl is LibreSSL, whose PKCS#12 format `security import` understands.
/usr/bin/openssl req -x509 -newkey rsa:2048 -nodes -days 3650 -config "$tmp/cert.cnf" \
	-keyout "$tmp/key.pem" -out "$tmp/cert.pem" 2>/dev/null
/usr/bin/openssl pkcs12 -export -inkey "$tmp/key.pem" -in "$tmp/cert.pem" \
	-out "$tmp/id.p12" -passout pass:podman-tray
# -T lets codesign use the key without a keychain prompt.
security import "$tmp/id.p12" -k "$KEYCHAIN" -P podman-tray -T /usr/bin/codesign >/dev/null

echo "Certificado \"$NAME\" criado. Os próximos builds serão assinados com ele."
