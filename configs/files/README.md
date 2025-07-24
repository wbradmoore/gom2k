# SSL/TLS Certificate Files

This directory contains SSL/TLS certificate files required for secure connections to Kafka and other services.

## Required Files for Kafka SSL Connection

Place the following files in this directory:

### Kafka SSL Files
- `kafka.truststore.jks` - Java KeyStore containing CA certificates to verify the Kafka server
- `kafka.keystore.jks` - Java KeyStore containing client certificate for mutual SSL authentication

### File Permissions
Ensure these files have restricted permissions:
```bash
chmod 600 configs/files/*.jks
```

## Configuration References

Update the paths in `kafka-config.yaml`:
```yaml
security:
  ssl:
    truststore:
      location: "./configs/files/kafka.truststore.jks"
      password: "your_truststore_password"
    keystore:
      location: "./configs/files/kafka.keystore.jks"
      password: "your_keystore_password"
      key_password: "your_key_password"
```

## Security Notes

- **DO NOT** commit these files to version control
- Add `*.jks` to `.gitignore`
- Use environment variables for passwords in production
- Rotate certificates regularly

## Alternative Authentication Methods

If you don't have JKS files, other supported methods include:
- SASL/PLAIN authentication
- SASL/SCRAM authentication  
- mTLS with PEM certificates
- Kerberos/GSSAPI

Contact your Kafka administrator for the appropriate authentication method and certificates.