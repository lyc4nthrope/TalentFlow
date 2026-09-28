# Reto 3 — Manual paso a paso

Cómo levantar el sistema, cómo ver el estado de cada componente y cómo ejecutar, en orden, todas las pruebas que pide `reto3.pdf` (sección 3). URL base del sistema: **`http://localhost:8080`**.

| Archivo | Para qué |
|---|---|
| **Este README** | Manual: levantar, consultar estados, probar cada requisito |
| [`demo.sh`](demo.sh) | Ejecuta automáticamente las pruebas 3.1, 3.2 y 3.3 |
| [`pruebas.md`](pruebas.md) | Evidencia: salida real de esas pruebas |
| [`bruno/`](bruno/) | Colección Bruno con la URL base del Gateway |
| [`guion-video.md`](guion-video.md) | Guion para grabar las capturas/video |

## 1. Prerrequisitos

- Docker con Docker Compose v2.
- El puerto **8080** libre en el host (es el único que se publica).
- Si vienes de una versión anterior del proyecto, empezar con volúmenes limpios: el esquema de empleados agregó la columna `validacion_departamento` en el Reto 3 y `init.sql` solo corre con el volumen vacío.

```bash
docker compose down -v
```

## 2. Levantar el sistema

```bash
docker compose up --build -d
docker compose ps
```

Esperado (tarda ~30 s en quedar todo `healthy`; el orden lo fuerza `depends_on: condition: service_healthy`: bases de datos → departamentos → empleados):

```
SERVICE                  STATUS                  PORTS
api-gateway              Up (healthy)            0.0.0.0:8080->8080/tcp
database-departamentos   Up (healthy)            3306/tcp, 33060/tcp
database-empleados       Up (healthy)            5432/tcp
departamentos-service    Up (healthy)            8082/tcp
empleados-service        Up (healthy)            8081/tcp
```

Solo `api-gateway` tiene `0.0.0.0:8080->…`: es el único puerto publicado. Los demás son internos (`expose:`).

## 3. ¿Dónde veo el estado de cada cosa?

### Todo el sistema en una sola petición: `GET /health` del Gateway

```bash
curl http://localhost:8080/health
```

```json
{
  "status": "UP",
  "service": "api-gateway",
  "timestamp": "2026-09-28T12:40:00.000Z",
  "sistema": "OK",
  "servicios": {
    "empleados-service": { "status": "UP", "app": "UP", "db": "UP", "circuitoDepartamentos": "CLOSED" },
    "departamentos-service": { "status": "UP", "app": "UP", "db": "UP" }
  }
}
```

| Campo | Qué significa |
|---|---|
| `status` | Salud **propia** del Gateway (`UP` mientras esté vivo; la respuesta es siempre `200`) |
| `sistema` | `OK` si todo está `UP` y el circuito `CLOSED`; si no, `DEGRADADO` |
| `servicios.<servicio>.status` | `UP`, o `DOWN` si su base de datos falla o si no responde (`"detalle": "No responde"`) |
| `servicios.<servicio>.db` | Estado de la base de datos **propia** de ese servicio |
| `servicios.empleados-service.circuitoDepartamentos` | Estado del Circuit Breaker: `CLOSED`, `OPEN` o `HALF_OPEN` |

Qué muestra ante cada fallo (verificado con caídas reales):

| Situación | Lo que reporta `GET /health` |
|---|---|
| Todo sano | `sistema: OK` |
| `departamentos-service` detenido | `departamentos-service: DOWN, "No responde"`; tras 4 registros fallidos, `circuitoDepartamentos: OPEN` |
| MySQL detenido (el servicio PHP sigue vivo) | `departamentos-service: DOWN, app: UP, db: DOWN` |
| `empleados-service` detenido | `empleados-service: DOWN, "No responde"` |
| Postgres detenido | `empleados-service: DOWN, app: UP, db: DOWN` |

> Un circuito `OPEN` **no** marca a `empleados-service` como `DOWN`: sigue atendiendo y registrando con el fallback `PENDIENTE` — degradado, pero vivo. Esa es justamente la protección del Circuit Breaker.

### Solo el Circuit Breaker

```bash
curl http://localhost:8080/empleados/circuito-departamentos
# {"dependencia":"departamentos-service","estado":"CLOSED"}
```

### Los contenedores

```bash
docker compose ps
```

Los cinco tienen `healthcheck`, así que cada uno muestra `(healthy)` o `(unhealthy)`. Docker decide tras varios chequeos seguidos: un servicio recién caído puede tardar ~25-30 s en pasar a `unhealthy` (el `GET /health` del Gateway lo refleja al instante).

### Los eventos, en vivo

```bash
docker compose logs -f empleados-service
```

```
⚠️ Circuit Breaker ABIERTO para Departamentos
🔄 Circuit Breaker HALF-OPEN para Departamentos
✅ Circuit Breaker CERRADO para Departamentos
🔁 Reconciliados N empleado(s) pendiente(s): [...]
```

## 4. Pruebas del reto, paso a paso

Automático: `bash docs/reto-03/demo.sh` ejecuta todo lo de esta sección en orden (~1 min 30 s). Manual:

### 4.1 Punto de entrada único (sección 3.1)

```bash
curl -i http://localhost:8080/departamentos   # 200 — a través del Gateway
curl -i http://localhost:8080/empleados       # 200
curl -i http://localhost:8081/departamentos   # DEBE FALLAR: conexión rechazada
curl -i http://localhost:8082/empleados       # DEBE FALLAR: conexión rechazada
curl -i http://localhost:8080/health          # 200 — health propio del Gateway
```

### 4.2 Manejo de errores del Gateway (sección 3.2)

```bash
docker compose stop departamentos-service
curl -i http://localhost:8080/departamentos
# HTTP/1.1 503 Service Unavailable
# Content-Type: application/json; charset=utf-8
# {"error":"Service Unavailable","message":"El servicio solicitado no está disponible temporalmente","path":"/departamentos"}
docker compose start departamentos-service
```

Esperar a que vuelva a estar `healthy` (`docker compose ps`) antes de seguir.

### 4.3 Circuit Breaker (sección 3.3)

> **Usar el modelo completo del empleado.** El PDF envía `{"id","nombre","departamentoId"}`; aquí el servicio exige el modelo canónico del Reto 1 y ese cuerpo recibe `400` inmediato por campos faltantes, **antes** de llegar al Circuit Breaker (no se vería ningún salto de tiempo).

**Paso 1 — Crear un departamento con el sistema sano**

```bash
curl -X POST http://localhost:8080/departamentos -H "Content-Type: application/json" \
  -d '{"id": "IT", "nombre": "Tecnología"}'
curl http://localhost:8080/empleados/circuito-departamentos     # CLOSED
```

**Paso 2 — Detener departamentos**

```bash
docker compose stop departamentos-service
```

**Paso 3 — Registrar 8 empleados y observar el tiempo**

```bash
for i in 1 2 3 4 5 6 7 8; do
  curl -s -o /dev/null -w "Petición $i -> HTTP %{http_code} | %{time_total}s\n" \
    -X POST http://localhost:8080/empleados -H "Content-Type: application/json" \
    -d "{\"id\":\"E00$i\",\"nombre\":\"Test\",\"apellido\":\"Reto3\",\"email\":\"e$i@reto3.test\",\"numeroEmpleado\":\"N00$i\",\"cargo\":\"Dev\",\"area\":\"IT\",\"departamentoId\":\"IT\",\"fechaIngreso\":\"2026-01-01\"}"
done
curl http://localhost:8080/empleados/circuito-departamentos     # OPEN
curl http://localhost:8080/empleados/E001                       # "validacionDepartamento":"PENDIENTE"
```

Esperado: peticiones 1-4 ≈ 0.6 s (circuito `CLOSED`, timeout + reintentos reales); desde la 5ª ≈ 0.01 s (circuito `OPEN`, fallback inmediato sin tocar la red). Todas `201` con `validacionDepartamento: "PENDIENTE"`.

**Paso 4 — Restaurar el servicio y esperar el timeout del circuito**

```bash
docker compose start departamentos-service
sleep 35
curl http://localhost:8080/empleados/circuito-departamentos     # HALF_OPEN
```

**Paso 5 — Recuperación automática**

```bash
curl -i -X POST http://localhost:8080/empleados -H "Content-Type: application/json" \
  -d '{"id":"E100","nombre":"Recuperado","apellido":"Reto3","email":"e100@reto3.test","numeroEmpleado":"N100","cargo":"Dev","area":"IT","departamentoId":"NO-EXISTE","fechaIngreso":"2026-01-01"}'
# 400 "El departamento NO-EXISTE no existe": solo puede venir de una consulta real
curl http://localhost:8080/empleados/circuito-departamentos     # CLOSED
```

**Paso 6 — Reconciliación automática de los pendientes**

```bash
curl http://localhost:8080/empleados/E001    # "validacionDepartamento":"ACEPTADO"
curl http://localhost:8080/health            # sistema: OK
```

Al cerrar el circuito, `empleados-service` revisa solo a todos los `PENDIENTE`: `ACEPTADO` si el departamento existe, `RECHAZADO` si no. Para ver un `RECHAZADO`, registrar en el paso 3 un empleado con `"departamentoId":"NO-EXISTE"` (con departamentos caído queda `PENDIENTE`; al reconciliar, `RECHAZADO`).

## 5. Colección Bruno y pruebas automatizadas

```bash
# Colección (con el sistema levantado): 10 peticiones, tests por petición
cd docs/reto-03/bruno && npx @usebruno/cli run --env Local

# Unitarias y de API de empleados (no requieren Docker)
npm test

# Integración de departamentos, vía Gateway (requieren el sistema levantado y PHP)
php microservicios/gestion-departamentos/test/test_departamentos.php
```

## 6. Problemas comunes

| Síntoma | Causa y solución |
|---|---|
| `column "validacion_departamento" does not exist` en los logs de empleados | Volumen de una versión anterior: `docker compose down -v` y volver a levantar |
| `Bind for 0.0.0.0:8080 failed: port is already allocated` | Otro proceso usa el 8080: detenerlo |
| Todas las peticiones de 4.3 responden `400` al instante | Se envió el cuerpo reducido del PDF: usar el modelo completo |
| Tras `sleep 35` el circuito sigue en `HALF_OPEN` | Es lo esperado: pasa a `HALF_OPEN` solo por tiempo (`resetTimeout`), pero necesita **una petición real** (paso 5) para probar la dependencia y pasar a `CLOSED` |
| `POST /departamentos` de IT responde `400` | Ya existía de una corrida anterior (los datos persisten en el volumen); se puede seguir |
