#!/bin/bash
# Generate long-lived test certificates using OpenSSL
set -e

CERTS_DIR="./certs"
mkdir -p "$CERTS_DIR"

echo "Generating long-lived test certificates with OpenSSL..."

cd "$CERTS_DIR"

# Generate CA private key
openssl genrsa -out ca.key 2048

# Generate CA certificate (valid for 10 years)
openssl req -new -x509 -days 3650 -key ca.key -out ca.crt -subj "/CN=Test CA"

# Generate server private key
openssl genrsa -out server.key 2048

# Create server certificate request with SAN extension
cat > server.conf <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = localhost

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = *.localhost
DNS.3 = test.local
DNS.4 = *.test.local
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

# Generate server certificate request
openssl req -new -key server.key -out server.csr -config server.conf

# Sign server certificate with CA (valid for 10 years)
openssl x509 -req -days 3650 -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -extensions v3_req -extfile server.conf

# Convert to Java KeyStore format for Kafka
# First convert to PKCS12
openssl pkcs12 -export -in server.crt -inkey server.key -out server.p12 -name kafka-server -password pass:testpass

# Convert PKCS12 to JKS for Kafka
keytool -importkeystore -srckeystore server.p12 -srcstoretype PKCS12 -srcstorepass testpass -destkeystore kafka.keystore.jks -deststoretype JKS -deststorepass testpass -destkeypass testpass >/dev/null 2>&1

# Create truststore with CA certificate
keytool -importcert -alias ca -keystore kafka.truststore.jks -storepass testpass -file ca.crt -noprompt >/dev/null 2>&1

# Create mkcert-compatible names for backward compatibility
cp server.crt localhost+1.pem
cp server.key localhost+1-key.pem

# Create symlinks for MQTT (use the same certs)
ln -sf server.crt mqtt-server.crt
ln -sf server.key mqtt-server.key
ln -sf ca.crt mqtt-ca.crt

# Clean up intermediate files
rm -f server.p12 server.csr server.conf ca.key ca.srl

# Set consistent permissions
chmod 644 *

echo "✅ Test certificates generated successfully with OpenSSL"
echo "📅 Certificates valid for 10 years"
echo "🔐 Self-signed certificates for testing only"