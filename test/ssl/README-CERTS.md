# SSL Test Certificates

## Overview
This directory contains streamlined SSL certificate generation for GOM2K testing.

## Quick Start
Certificates are generated automatically when running SSL tests:
```bash
cd test && ./test.sh ssl
```

## How It Works

### Automatic Generation
- Certificates are generated automatically when needed
- If certificates exist and are less than 30 days old, they are reused
- No manual intervention required for normal testing

### Manual Regeneration
If you need to regenerate certificates manually:
```bash
cd test/ssl
./regenerate-certs.sh
```

## Certificate Details

### Generated Files
- `kafka.keystore.jks` - Kafka server keystore (password: testpass)
- `kafka.truststore.jks` - Kafka truststore (password: testpass)
- `ca.crt` - Certificate Authority certificate
- `mqtt-ca.crt` - MQTT CA certificate
- `mqtt-server.crt` - MQTT server certificate
- `mqtt-server.key` - MQTT server private key

### Key Features
- **Valid for 365 days** - Long enough for development
- **Subject Alternative Names (SAN)** - Works with modern TLS requirements
- **localhost + 127.0.0.1** - Supports both hostname and IP access
- **Consistent passwords** - All keystores use "testpass" for simplicity

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