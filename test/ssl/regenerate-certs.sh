#!/bin/bash
# Force regeneration of test certificates
# Usage: ./regenerate-certs.sh

set -e

CERTS_DIR="./certs"

echo "Force regenerating test certificates..."

# Remove existing certificates
rm -rf "$CERTS_DIR"
mkdir -p "$CERTS_DIR"

# Run the generation script (will create new certificates)
./generate-certs.sh

echo "Certificates regenerated successfully."
echo "New certificates are valid for 365 days."