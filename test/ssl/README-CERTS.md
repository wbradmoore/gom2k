# SSL Test Certificates

## Overview
This directory contains simplified SSL certificate generation using **mkcert** for GOM2K testing.

## Prerequisites (Required)
**mkcert** is required for SSL certificate generation. Install from your system package manager:

```bash
# macOS
brew install mkcert

# Linux (Ubuntu/Debian)
sudo apt install libnss3-tools
# Then download mkcert binary from: https://github.com/FiloSottile/mkcert#linux

# Or use manual installation: https://github.com/FiloSottile/mkcert#installation
```

## Quick Start
Certificates are generated automatically when running SSL tests:
```bash
cd test && ./test.sh ssl
```

## How It Works

### Automatic Generation with mkcert
- Uses **mkcert** for simplified, trusted certificate generation
- Automatically installs local CA on first run
- Certificates are trusted by your system (no browser warnings)
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
- `localhost+1.pem` - Server certificate (mkcert format)
- `localhost+1-key.pem` - Server private key (mkcert format)
- `kafka.keystore.jks` - Kafka server keystore (password: testpass)
- `kafka.truststore.jks` - Kafka truststore (password: testpass)
- `ca.crt` - Certificate Authority certificate
- `mqtt-server.crt` - MQTT server certificate (symlink)
- `mqtt-server.key` - MQTT server private key (symlink)
- `mqtt-ca.crt` - MQTT CA certificate (symlink)

### Key Features
- **System trusted** - No browser warnings or manual CA installation
- **mkcert managed** - Automatically handles CA creation and installation
- **Subject Alternative Names (SAN)** - Works with modern TLS requirements
- **localhost + 127.0.0.1** - Supports both hostname and IP access
- **Consistent passwords** - All keystores use "testpass" for simplicity
- **Single source** - Same certificate used for both Kafka and MQTT

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