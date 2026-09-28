# Reto 3 — Evidencia de pruebas

Resultados reales de las pruebas de la sección 3 de `reto3.pdf`, obtenidos con el sistema levantado desde cero (`docker compose up --build`, volúmenes vacíos) sobre la rama `lyc4nthrope`, el 2026-09-28.

| Archivo | Qué es |
|---|---|
| [`demo.sh`](demo.sh) | Ejecuta las pruebas 3.1, 3.2 y 3.3 del reto en orden. La salida de abajo es la de este script, sin editar. |
| [`bruno/`](bruno/) | Colección Bruno apuntando a la URL base del sistema (`http://localhost:8080`), con tests por petición. |
| [`guion-video.md`](guion-video.md) | Guion para grabar las capturas/video que exige el reto. |

Cómo reproducir todo:

```bash
docker compose down -v            # opcional: empezar con volúmenes limpios
docker compose up --build -d
bash docs/reto-03/demo.sh
```

> **Diferencia deliberada con el guion literal del PDF.** La sección 3.3 del PDF envía `{"id","nombre","departamentoId"}`. Desde el Reto 1 el servicio exige el modelo canónico completo (`apellido`, `email`, `numeroEmpleado`, `cargo`, `area`, `fechaIngreso`), así que ese cuerpo responde `400` al instante por validación local — **antes** de llamar a `departamentos-service` — y nunca ejercita el Circuit Breaker (verificado: 3 peticiones → `400` en ~0.005s cada una, sin salto de tiempo). `demo.sh` envía el modelo completo; todo lo demás sigue el PDF paso a paso.

## Contenedores — solo el Gateway publica puerto

```
SERVICE                  STATUS                    PORTS
api-gateway              Up 5 seconds              0.0.0.0:8080->8080/tcp, [::]:8080->8080/tcp
database-departamentos   Up 22 seconds (healthy)   3306/tcp, 33060/tcp
database-empleados       Up 21 seconds (healthy)   5432/tcp
departamentos-service    Up 11 seconds (healthy)   8082/tcp
empleados-service        Up 5 seconds              8081/tcp
```

Solo `api-gateway` tiene mapeo `0.0.0.0:…->…`; el resto son puertos internos (`expose:`).

## 3.1 Punto de entrada único

```
GET http://localhost:8080/departamentos      -> HTTP 200
GET http://localhost:8080/empleados          -> HTTP 200
GET http://localhost:8080/health             -> HTTP 200
GET http://localhost:8081/departamentos      -> conexión rechazada (curl exit 7)
GET http://localhost:8082/empleados          -> conexión rechazada (curl exit 7)
GET http://localhost:8080/health -> {"status":"UP","service":"api-gateway"}
```

✅ Por el Gateway funciona; el acceso directo a los microservicios se rechaza (`curl` exit 7 = conexión rechazada).

## 3.2 Manejo de errores del Gateway

```
departamentos-service detenido
HTTP/1.1 503 Service Unavailable
Content-Type: application/json; charset=utf-8
{"error":"Service Unavailable","message":"El servicio solicitado no está disponible temporalmente","path":"/departamentos"}
departamentos-service restaurado (healthy)
```

✅ `503` con cuerpo JSON descriptivo, no un stack trace ni la página de error del framework.

## 3.3 Circuit Breaker

### Pasos 1-3: salto en el tiempo de respuesta

```
POST /departamentos {"id":"IT"} -> HTTP 201
Estado del circuito: {"dependencia":"departamentos-service","estado":"CLOSED"}

departamentos-service detenido

Petición 1 -> HTTP 201 | 0.656134s
Petición 2 -> HTTP 201 | 0.627882s
Petición 3 -> HTTP 201 | 0.628250s
Petición 4 -> HTTP 201 | 0.627210s
Petición 5 -> HTTP 201 | 0.008934s
Petición 6 -> HTTP 201 | 0.007841s
Petición 7 -> HTTP 201 | 0.008099s
Petición 8 -> HTTP 201 | 0.008214s
Estado del circuito: {"dependencia":"departamentos-service","estado":"OPEN"}

Petición R071759BAD (departamentoId NO-EXISTE) -> HTTP 201 | 0.008648s
GET /empleados/R07175901 -> "validacionDepartamento":"PENDIENTE"
```

✅ **Ese es el salto:** las peticiones 1-4 tardan ~0.63s (circuito `CLOSED`: timeout + 3 reintentos reales); desde la 5ª, ~0.008s (circuito `OPEN`: fallback inmediato, sin tocar la red) — unas 80 veces más rápido. Abre exactamente tras 4 fallos porque `volumeThreshold = 4` (dentro de los "3-5 fallos" que sugiere el reto).

✅ **Fallback:** todas responden `201` con `validacionDepartamento: "PENDIENTE"` — RR. HH. no queda bloqueado. La petición extra con un departamento inexistente también queda `PENDIENTE`: con `departamentos-service` caído no hay forma de saber que no existe; se decide al reconciliar.

### Pasos 4-5: recuperación automática

```
departamentos-service restaurado; sleep 35 (resetTimeout = 30s)
Estado del circuito: {"dependencia":"departamentos-service","estado":"HALF_OPEN"}

{"status":400,"error":"Bad Request","message":"El departamento NO-EXISTE no existe","timestamp":"2026-09-28T12:19:11.307Z","path":"/empleados","errors":[{"field":"departamentoId","message":"No existe","rejectedValue":"NO-EXISTE"}]}
-> HTTP 400
Estado del circuito: {"dependencia":"departamentos-service","estado":"CLOSED"}
```

✅ Tras el `resetTimeout` el circuito pasa a `HALF_OPEN`; la siguiente petición real es la llamada de prueba, consulta de verdad a `departamentos-service` y responde `400` — un `400` solo puede venir de una consulta real, no del fallback. El circuito cierra solo, sin reiniciar nada. **Los tres estados quedan demostrados** (`CLOSED` → `OPEN` → `HALF_OPEN` → `CLOSED`).

### Reconciliación automática de los pendientes

```
GET /empleados/R07175901      -> "departamentoId":"IT" "validacionDepartamento":"ACEPTADO"
GET /empleados/R07175908      -> "departamentoId":"IT" "validacionDepartamento":"ACEPTADO"
GET /empleados/R071759BAD     -> "departamentoId":"NO-EXISTE" "validacionDepartamento":"RECHAZADO"

Log de empleados-service:
⚠️ Circuit Breaker ABIERTO para Departamentos
🔄 Circuit Breaker HALF-OPEN para Departamentos
✅ Circuit Breaker CERRADO para Departamentos
🔁 Reconciliados 9 empleado(s) pendiente(s): [
```

✅ Al cerrar el circuito, los 9 pendientes se revisaron solos: los de un departamento existente quedaron `ACEPTADO` y el de un departamento inexistente, `RECHAZADO`. Ninguna intervención manual.

## Colección de pruebas (Bruno)

Con el sistema levantado:

```bash
cd docs/reto-03/bruno
npx @usebruno/cli run --env Local
```

Resultado (dos corridas consecutivas sobre los mismos volúmenes, ambas iguales):

```
Status        ✓ PASS
Requests      10 (10 Passed)
Tests         11/11
```

También se abre desde la app de Bruno: *Open Collection* → `docs/reto-03/bruno`, entorno `Local`.

## Pruebas automatizadas

| Suite | Comando | Resultado |
|---|---|---|
| Unitarias + API de empleados (Node) | `npm test` | 30/30 |
| Integración de departamentos (PHP, vía Gateway) | `php microservicios/gestion-departamentos/test/test_departamentos.php` | 17/17 |
