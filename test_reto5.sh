#!/usr/bin/env bash
set -eo pipefail

export GATEWAY="http://localhost:8080"
export HOY=$(date +%Y-%m-%d)

echo "=================================================="
echo "      PRUEBAS AUTOMATIZADAS - RETO 5 (JWT & RBAC)"
echo "=================================================="
echo

echo "⌛ Esperando a que el API Gateway en $GATEWAY esté disponible..."
until curl -s "$GATEWAY" > /dev/null 2>&1 || curl -s "$GATEWAY/health" > /dev/null 2>&1; do
  printf '.'
  sleep 2
done
echo -e "\n✅ API Gateway respondiendo.\n"

echo "=== 1) Login del Administrador (Semilla) ==="
export ADMIN_EMAIL="admin@empresa.com"
export ADMIN_PASS="Admin123*"

export ADMIN_TOKEN=$(curl -sS -f -X POST "$GATEWAY/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASS\"}" \
  | python3 -c 'import sys, json; d=json.load(sys.stdin); print(d.get("token",""))')

if [ -z "$ADMIN_TOKEN" ]; then
  echo "❌ Error: No se pudo obtener ADMIN_TOKEN"
  exit 1
fi

echo "✅ ADMIN_TOKEN Obtenido: ${ADMIN_TOKEN:0:30}..."
echo

echo "=== 2) Petición Denegada sin Token (401 Unauthorized) ==="
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$GATEWAY/empleados")
if [ "$HTTP_CODE" -eq 401 ]; then
  echo "✅ Código 401 recibido correctamente al no enviar token."
else
  echo "❌ Error: Se esperaba 401 y se obtuvo $HTTP_CODE"
  exit 1
fi
echo

echo "=== 3) Crear/Asegurar Departamento IT (ADMIN) ==="
curl -sS -f -X POST "$GATEWAY/departamentos" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"id":"IT","nombre":"Tecnología","descripcion":"Departamento de TI"}' > /dev/null || true
echo "✅ Departamento IT verificado/creado."
echo

echo "=== 4) Onboarding: Crear Empleado Nuevo (ADMIN) ==="
export EMP_ID="E2026-$(date +%s)"
export EMP_EMAIL="ana.nueva.$RANDOM@empresa.com"
export EMP_NUM="EMP-2026-$(date +%s)"

echo "EMP_ID=$EMP_ID"
echo "EMP_EMAIL=$EMP_EMAIL"

curl -sS -f -X POST "$GATEWAY/empleados" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  --data "{
    \"id\":\"$EMP_ID\",
    \"nombre\":\"Ana\",
    \"apellido\":\"Nueva\",
    \"email\":\"$EMP_EMAIL\",
    \"numeroEmpleado\":\"$EMP_NUM\",
    \"cargo\":\"Developer\",
    \"area\":\"TI\",
    \"departamentoId\":\"IT\",
    \"fechaIngreso\":\"$HOY\",
    \"estado\":\"ACTIVO\"
  }" > /dev/null

echo "✅ Empleado creado. Evento 'empleado.creado' publicado."
sleep 2 # Dar tiempo a que el auth-service procese el evento
echo

echo "=== 5) Verificar Usuario Inactivo y Extraer Token de Activación ==="
export RESET_TOKEN=$(docker compose exec -T database-auth psql -U user_auth -d db_auth -At -c \
  "SELECT token_recuperacion FROM usuarios WHERE LOWER(email)=LOWER('$EMP_EMAIL');" | tr -d '\r\n')

if [ -z "$RESET_TOKEN" ]; then
  echo "❌ Error: El usuario no fue creado en auth-service o no tiene token."
  exit 1
fi

echo "✅ Token de activación capturado: ${RESET_TOKEN:0:25}..."
echo

echo "=== 6) Establecer Contraseña Inicial (/auth/reset-password) ==="
export USER_PASS="NuevaClave123!"

curl -sS -f -X POST "$GATEWAY/auth/reset-password" \
  -H "Content-Type: application/json" \
  --data "{\"token\":\"$RESET_TOKEN\",\"newPassword\":\"$USER_PASS\"}" > /dev/null

echo "✅ Contraseña establecida exitosamente."
echo

echo "=== 7) Login del Usuario Normal (USER) y Obtener JWT ==="
export USER_TOKEN=$(curl -sS -f -X POST "$GATEWAY/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMP_EMAIL\",\"password\":\"$USER_PASS\"}" \
  | python3 -c 'import sys, json; d=json.load(sys.stdin); print(d.get("token",""))')

if [ -z "$USER_TOKEN" ]; then
  echo "❌ Error: No se pudo hacer login con el nuevo usuario."
  exit 1
fi

echo "✅ USER_TOKEN Obtenido: ${USER_TOKEN:0:30}..."
echo

echo "=== 8) Lectura Permitida a Rol USER (200 OK) ==="
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $USER_TOKEN" "$GATEWAY/empleados")
if [ "$HTTP_CODE" -eq 200 ]; then
  echo "✅ Acceso de lectura exitoso con USER_TOKEN."
else
  echo "❌ Error: Se esperaba 200 y se obtuvo $HTTP_CODE"
  exit 1
fi
echo

echo "=== 9) Prueba RBAC: Escalamiento Vertical Denegado (403 Forbidden) ==="
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$GATEWAY/empleados" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"id":"E999","nombre":"Mallory","apellido":"Intruso","email":"mallory@empresa.com"}')

if [ "$HTTP_CODE" -eq 403 ]; then
  echo "✅ Creación denegada a USER (403 Forbidden) correctamente."
else
  echo "❌ Error: Se esperaba 403 Forbidden y se obtuvo $HTTP_CODE"
  exit 1
fi
echo

echo "=== 10) Prueba ABAC (Propiedad de Recurso): Perfil Propio vs Ajeno ==="
HTTP_CODE_PROPIO=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$GATEWAY/perfiles/$EMP_ID" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"direccion":"Calle 123"}')
echo "  - Perfil propio ($EMP_ID): HTTP $HTTP_CODE_PROPIO"

HTTP_CODE_AJENO=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$GATEWAY/perfiles/E001" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"direccion":"Calle Hack"}')

if [ "$HTTP_CODE_AJENO" -eq 403 ]; then
  echo "✅ Edición de perfil ajeno denegada (403 Forbidden) correctamente."
else
  echo "❌ Error ABAC: Se esperaba 403 y se obtuvo $HTTP_CODE_AJENO"
  exit 1
fi
echo

echo "=================================================="
echo "   🎉 TODAS LAS PRUEBAS FINALIZARON EXITOSAMENTE"
echo "=================================================="