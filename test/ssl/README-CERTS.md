# SSL Test Certificates

## Overview
This directory contains long-lived SSL test certificates for GOM2K testing. The certificates are pre-generated and committed to the repository for convenience.

## Prerequisites (Only for regeneration)
If you need to regenerate certificates, you'll need:
- **OpenSSL** for certificate generation
- **Java keytool** for JKS keystore creation

## Quick Start
The certificates are already generated and committed to the repository. Just run tests:
```bash
cd test && ./test.sh ssl
```

## How It Works

### Pre-generated Certificates
- Long-lived certificates (valid for 10 years) committed to repository
- No external dependencies required for testing
- Self-signed certificates for testing only
- Certificates are automatically used by tests

### Manual Regeneration
If you need to regenerate certificates manually:
```bash
cd test/ssl
./regenerate-certs.sh
```

## Certificate Details

### Certificate Files
- `server.crt` - Server certificate (PEM format)
- `server.key` - Server private key (PEM format)
- `localhost+1.pem` - Server certificate (mkcert-compatible name)
- `localhost+1-key.pem` - Server private key (mkcert-compatible name)
- `kafka.keystore.jks` - Kafka server keystore (password: testpass)
- `kafka.truststore.jks` - Kafka truststore (password: testpass)
- `ca.crt` - Certificate Authority certificate
- `mqtt-server.crt` - MQTT server certificate (symlink)
- `mqtt-server.key` - MQTT server private key (symlink)
- `mqtt-ca.crt` - MQTT CA certificate (symlink)

### Key Features
- **Long-lived certificates** - Valid for 10 years (no expiration issues)
- **Pre-committed** - No external tools required for testing
- **Subject Alternative Names (SAN)** - Works with modern TLS requirements
- **Multiple hostnames** - Supports localhost, *.localhost, test.local, *.test.local
- **IP addresses** - Supports 127.0.0.1 and ::1
- **Consistent passwords** - All keystores use "testpass" for simplicity
- **Multiple formats** - Both PEM and JKS formats available

## Troubleshooting

### Certificate Errors
If you see certificate-related errors:
```bash
cd test/ssl
./regenerate-certs.sh
cd .. && ./test.sh ssl
```

### Clean Start
To completely clean and regenerate:
```bash
rm -rf test/ssl/certs/
cd test && ./test.sh ssl
```

## Security Notes
- **Test certificates only** - Never use these certificates in production
- **Weak passwords** - "testpass" is intentionally simple for testing
- **Self-signed** - These certificates are not signed by a real CA
- **Short validity** - Certificates expire after 365 days