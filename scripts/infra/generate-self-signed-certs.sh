#!/usr/bin/env bash
# Generate self-signed CA and certificates for local development
# Usage: ./scripts/infra/generate-self-signed-certs.sh [--output-dir <dir>] [--ca-name <name>] [--domains <domain1,domain2>]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

OUTPUT_DIR="${PROJECT_ROOT}/infra/secrets/certs"
CA_NAME="ai-aas-ca"
DOMAINS=(
  "api.dev.ai-aas.local"
  "portal.dev.ai-aas.local"
  "grafana.dev.ai-aas.local"
  "argocd.dev.ai-aas.local"
  "api.prod.ai-aas.local"
  "portal.prod.ai-aas.local"
  "grafana.prod.ai-aas.local"
  "argocd.prod.ai-aas.local"
)

while [[ $# -gt 0 ]]; do
  case "$1" in
    --output-dir)
      OUTPUT_DIR="$2"
      shift 2
      ;;
    --ca-name)
      CA_NAME="$2"
      shift 2
      ;;
    --domains)
      IFS=',' read -ra DOMAINS <<< "$2"
      shift 2
      ;;
    --help|-h)
      cat <<USAGE
Usage: $(basename "$0") [--output-dir <dir>] [--ca-name <name>] [--domains <domain1,domain2>]

Generates a self-signed CA and certificates for all specified domains.

Options:
  --output-dir    Directory to store certificates (default: infra/secrets/certs)
  --ca-name       Name for the CA certificate (default: ai-aas-ca)
  --domains       Comma-separated list of domains (default: all dev/prod endpoints)

Examples:
  # Generate with defaults
  $0

  # Generate for specific domains
  $0 --domains "api.dev.ai-aas.local,portal.dev.ai-aas.local"

  # Custom output directory
  $0 --output-dir /tmp/certs
USAGE
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
done

# Create output directory
mkdir -p "${OUTPUT_DIR}"

echo "🔐 Generating self-signed certificates..."
echo "   Output directory: ${OUTPUT_DIR}"
echo "   CA name: ${CA_NAME}"
echo "   Domains: ${DOMAINS[*]}"
echo ""

# Generate CA private key
echo "📝 Step 1: Generating CA private key..."
openssl genrsa -out "${OUTPUT_DIR}/${CA_NAME}.key" 4096

# Generate CA certificate
echo "📝 Step 2: Generating CA certificate..."
openssl req -new -x509 -days 3650 -key "${OUTPUT_DIR}/${CA_NAME}.key" \
  -out "${OUTPUT_DIR}/${CA_NAME}.crt" \
  -subj "/CN=${CA_NAME}/O=AI-AAS/C=US" \
  -addext "basicConstraints=critical,CA:true"

# Generate wildcard certificate for all domains
echo "📝 Step 3: Generating wildcard certificate for all domains..."

# Create certificate config
CERT_CONFIG="${OUTPUT_DIR}/cert.conf"
cat > "${CERT_CONFIG}" <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = *.ai-aas.local
O = AI-AAS
C = US

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
EOF

# Add all domains to SAN
for i in "${!DOMAINS[@]}"; do
  echo "DNS.$((i+1)) = ${DOMAINS[$i]}" >> "${CERT_CONFIG}"
done

# Generate private key for certificate
openssl genrsa -out "${OUTPUT_DIR}/tls.key" 2048

# Generate certificate signing request
openssl req -new -key "${OUTPUT_DIR}/tls.key" \
  -out "${OUTPUT_DIR}/tls.csr" \
  -config "${CERT_CONFIG}"

# Sign certificate with CA
openssl x509 -req -days 365 \
  -in "${OUTPUT_DIR}/tls.csr" \
  -CA "${OUTPUT_DIR}/${CA_NAME}.crt" \
  -CAkey "${OUTPUT_DIR}/${CA_NAME}.key" \
  -CAcreateserial \
  -out "${OUTPUT_DIR}/tls.crt" \
  -extensions v3_req \
  -extfile "${CERT_CONFIG}"

# Clean up CSR and config
rm -f "${OUTPUT_DIR}/tls.csr" "${OUTPUT_DIR}/cert.conf"

echo ""
echo "✅ Certificates generated successfully!"
echo ""
echo "📋 Files created:"
echo "   CA Certificate: ${OUTPUT_DIR}/${CA_NAME}.crt"
echo "   CA Private Key: ${OUTPUT_DIR}/${CA_NAME}.key"
echo "   TLS Certificate: ${OUTPUT_DIR}/tls.crt"
echo "   TLS Private Key: ${OUTPUT_DIR}/tls.key"
echo ""
echo "🔐 Next steps:"
echo "   1. Trust the CA certificate on your local machine:"
echo "      Linux:   sudo cp ${OUTPUT_DIR}/${CA_NAME}.crt /usr/local/share/ca-certificates/${CA_NAME}.crt && sudo update-ca-certificates"
echo "      macOS:   sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ${OUTPUT_DIR}/${CA_NAME}.crt"
echo "      Windows: Import ${OUTPUT_DIR}/${CA_NAME}.crt into Trusted Root Certification Authorities"
echo ""
echo "   2. Create Kubernetes secrets:"
echo "      ./scripts/infra/create-tls-secrets.sh --cert-dir ${OUTPUT_DIR}"
echo ""
echo "   3. Update hosts file:"
echo "      ./scripts/infra/update-hosts-file.sh"

