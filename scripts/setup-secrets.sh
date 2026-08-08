#!/usr/bin/env bash
# setup-secrets.sh
#
# Genera la configuración local de secretos (una sola vez, después de clonar/descomprimir).
# NO versiona secretos: solo crea los archivos locales excluidos por .gitignore.
#
# Uso:
#   bash scripts/setup-secrets.sh
#
# En el Reto 1 no hay secretos de servicios todavía; este script prepara la base
# para los retos 3+ (broker, bases de datos) y 9-10 (gestión de secretos).

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${ROOT_DIR}/.env"

if [[ ! -f "${ENV_FILE}" ]]; then
  cp "${ROOT_DIR}/.env.example" "${ENV_FILE}"
  echo "✓ Creado .env a partir de .env.example"
else
  echo "· .env ya existe, no se sobrescribe"
fi

# Directorio donde vivirán los secretos reales (excluido por .gitignore)
mkdir -p "${ROOT_DIR}/secrets"
echo "✓ Directorio secrets/ listo (excluido de git)"

echo "Configuración local lista."
