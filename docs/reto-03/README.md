# Reto 3 — Manual paso a paso

Cómo levantar el sistema, cómo ver el estado de cada componente y cómo demostrar, en orden, todo lo que pide `reto3.pdf`. URL base del sistema: **`http://localhost:8080`**.

| Archivo | Para qué |
|---|---|
| **Este README** | Manual: requisitos del reto, estados, pruebas paso a paso y respuestas para la sustentación |
| [`demo.sh`](demo.sh) | Ejecuta automáticamente las pruebas 3.1, 3.2 y 3.3 del PDF |
| [`pruebas.md`](pruebas.md) | Evidencia: salida real de esas pruebas y de cada escenario de caída |
| [`bruno/`](bruno/) | Colección Bruno con la URL base del Gateway |
| [`guion-video.md`](guion-video.md) | Guion para grabar las capturas/video |
| [README raíz](../../README.md) | Documento de entrega: justificaciones completas y decisiones técnicas |

## 0. Criterios de evaluación → dónde se demuestra

| # | Criterio (valor) | Qué pide el PDF | Dónde se demuestra |
|---|---|---|---|
| 1 | API Gateway funcional (1.5) | Gateway de aplicación en Compose; enruta a ambos servicios propagando cuerpo, cabeceras y código; `503` JSON si el destino no responde; `/health` propio | [§4.1](#41-enrutamiento-y-propagación-sección-14), [§4.3](#43-manejo-de-errores-del-gateway-sección-32), [§3](#3-dónde-veo-el-estado-de-cada-cosa) |
| 2 | Punto de entrada único (1.0) | Solo el Gateway publica puertos (`expose:` en los servicios); acceso directo rechazado; README y colección con la nueva URL base | [§2](#2-levantar-el-sistema), [§4.2](#42-punto-de-entrada-único-secciones-15-y-31), [§6](#6-colección-bruno-y-pruebas-automatizadas) |
| 3 | Circuit Breaker (1.5) | Librería del ecosistema en `empleados → departamentos`; tres estados demostrables; salto de tiempo; recuperación sin reinicio | [§4.4](#44-circuit-breaker-sección-33), [§5.2](#52-parámetros-del-circuit-breaker-sección-23) |
| 4 | Estrategia de fallback (0.5) | Fallback funcional; decisión justificada (disponibilidad vs. consistencia); reconciliación explicada | [§4.4 paso 6](#44-circuit-breaker-sección-33), [§5.3](#53-fallback-y-reconciliación-sección-26) |
| 5 | Documentación y evidencias (0.5) | README con tabla de rutas y parámetros; capturas de las tres pruebas | [§5.1](#51-tabla-de-rutas-del-gateway), [`pruebas.md`](pruebas.md), [`guion-video.md`](guion-video.md) |

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

Solo `api-gateway` tiene `0.0.0.0:8080->…`: es el único puerto publicado. Los demás son internos (`expose:`, ver `docker-compose.yml`).

## 3. ¿Dónde veo el estado de cada cosa?

| Qué quiero saber | Comando | Respuesta |
|---|---|---|
| **Todo el sistema de una vez** | `curl http://localhost:8080/health` | Gateway + cada servicio + su BD + Circuit Breaker |
| Solo el Circuit Breaker | `curl http://localhost:8080/empleados/circuito-departamentos` | `CLOSED` / `OPEN` / `HALF_OPEN` |
| `/health` interno de departamentos | `docker compose exec departamentos-service php -r 'echo file_get_contents("http://127.0.0.1:8082/health"), PHP_EOL;'` | `status` + `components.app` / `components.db` (MySQL) |
| `/health` interno de empleados | `docker compose exec empleados-service node -e "fetch('http://127.0.0.1:8081/health').then(r=>r.text()).then(console.log)"` | `status` + `components.app` / `db` (Postgres) / `circuitoDepartamentos` |
| Los 5 contenedores | `docker compose ps` | `(healthy)` / `(unhealthy)` en cada uno |
| Los eventos del circuito en vivo | `docker compose logs -f empleados-service` | `ABIERTO` / `HALF-OPEN` / `CERRADO` / `Reconciliados N` |

Los `/health` de cada servicio son **internos** (el Gateway solo enruta `/empleados/*` y `/departamentos/*`); desde fuera se ven a través del `/health` del Gateway, que los consulta por la red de Docker.

### `GET /health` del Gateway

```bash
curl http://localhost:8080/health
```

```json
{
  "status": "UP",
  "service": "api-gateway",
  "timestamp": "2026-09-28T12:33:05.819Z",
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

### Qué muestra ante cada fallo (verificado con caídas reales)

| Situación | `GET /health` del Gateway | `docker compose ps` |
|---|---|---|
| Todo sano | `sistema: OK` | 5/5 `(healthy)` |
| `docker compose stop departamentos-service` | `departamentos-service: DOWN, "No responde"`; tras 4 registros fallidos, `circuitoDepartamentos: OPEN` | contenedor `Exited` |
| `docker compose stop database-departamentos` (PHP sigue vivo) | `departamentos-service: DOWN, app: UP, db: DOWN` | `ms-departamentos` → `unhealthy` (~25 s) |
| `docker compose stop empleados-service` | `empleados-service: DOWN, "No responde"` | contenedor `Exited` |
| `docker compose stop database-empleados` | `empleados-service: DOWN, app: UP, db: DOWN` | `ms-empleados` → `unhealthy` (~25 s) |
| Todo restaurado (`docker compose start …`) | vuelve a `sistema: OK` | 5/5 `(healthy)` |

> Un circuito `OPEN` **no** marca a `empleados-service` como `DOWN`: sigue atendiendo y registrando con el fallback `PENDIENTE` — degradado, pero vivo. Esa es justamente la protección del Circuit Breaker.
>
> `docker compose ps` tarda ~25 s en marcar `unhealthy` porque Docker exige varios chequeos fallidos seguidos; el `/health` del Gateway lo refleja al instante.

## 4. Pruebas del reto, paso a paso

Automático: `bash docs/reto-03/demo.sh` ejecuta §4.2, §4.3 y §4.4 en orden (~1 min 30 s). Manual:

### 4.1 Enrutamiento y propagación (sección 1.4)

El Gateway reenvía cuerpo, cabeceras y código de estado **sin alterarlos**:

```bash
curl -i -X POST http://localhost:8080/departamentos -H "Content-Type: application/json" \
  -d '{"id":"RRHH","nombre":"Recursos Humanos"}'
# HTTP/1.1 201 Created                       <- código del servicio
# location: /departamentos/RRHH              <- cabecera del servicio
# {"id":"RRHH","nombre":"Recursos Humanos","descripcion":null}

curl -i http://localhost:8080/departamentos/RRHH
# HTTP/1.1 200 OK
# x-powered-by: PHP/8.2.34                   <- solo puede venir del servicio PHP

curl -i http://localhost:8080/departamentos/NOPE
# HTTP/1.1 404 Not Found                     <- 404 del servicio, con su cuerpo, no del Gateway
# {"status":404,"error":"Not Found","message":"El departamento con id NOPE no existe",...}

curl -i http://localhost:8080/ruta-inexistente
# HTTP/1.1 404 Not Found                     <- ruta no enrutada: responde el propio Gateway, en JSON
# {"error":"Not Found","message":"Ruta no enrutada por el API Gateway","path":"/ruta-inexistente"}
```

### 4.2 Punto de entrada único (secciones 1.5 y 3.1)

```bash
curl -i http://localhost:8080/departamentos   # 200 — a través del Gateway
curl -i http://localhost:8080/empleados       # 200
curl -i http://localhost:8080/health          # 200 — health propio del Gateway

curl -i http://localhost:8081/empleados       # DEBE FALLAR: conexión rechazada (verificación obligatoria 1.5)
curl -i http://localhost:8081/departamentos   # DEBE FALLAR (3.1)
curl -i http://localhost:8082/empleados       # DEBE FALLAR (3.1)
curl -i http://localhost:8082/departamentos   # DEBE FALLAR
```

Las cuatro directas responden `curl: (7) Failed to connect … Connection refused`: nada escucha en el host fuera del 8080.

### 4.3 Manejo de errores del Gateway (sección 3.2)

```bash
docker compose stop departamentos-service
curl -i http://localhost:8080/departamentos
# HTTP/1.1 503 Service Unavailable
# Content-Type: application/json; charset=utf-8
# {"error":"Service Unavailable","message":"El servicio solicitado no está disponible temporalmente","path":"/departamentos"}
curl http://localhost:8080/health             # sistema: DEGRADADO, departamentos-service: DOWN
docker compose start departamentos-service
```

Esperar a que vuelva a estar `healthy` (`docker compose ps`) antes de seguir.

### 4.4 Circuit Breaker (sección 3.3)

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
curl http://localhost:8080/health                               # circuitoDepartamentos: OPEN
```

Esperado: peticiones 1-4 ≈ 0.6 s (circuito `CLOSED`, timeout + reintentos reales); desde la 5ª ≈ 0.01 s (circuito `OPEN`, fallback inmediato sin tocar la red). Todas `201` con `validacionDepartamento: "PENDIENTE"`. **Ese salto es la evidencia central del reto.**

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

Nadie reinició nada: el circuito probó la dependencia en `HALF_OPEN`, funcionó y cerró. Los tres estados quedan demostrados.

**Paso 6 — Reconciliación automática de los pendientes**

```bash
curl http://localhost:8080/empleados/E001    # "validacionDepartamento":"ACEPTADO"
curl http://localhost:8080/health            # sistema: OK
```

Al cerrar el circuito, `empleados-service` revisa solo a todos los `PENDIENTE`: `ACEPTADO` si el departamento existe, `RECHAZADO` si no (en los logs: `🔁 Reconciliados N empleado(s)`). Para ver un `RECHAZADO`, registrar en el paso 3 un empleado con `"departamentoId":"NO-EXISTE"`: con departamentos caído queda `PENDIENTE`; al reconciliar, `RECHAZADO`.

## 5. Decisiones que el reto pide justificar

Resumen; la justificación completa está en el [README raíz](../../README.md).

### 5.1 Tabla de rutas del Gateway

| Ruta externa | Servicio interno |
|---|---|
| `GET /health` | el propio Gateway (salud propia + estado del sistema) |
| `/empleados/*` | `http://empleados-service:8081` |
| `/departamentos/*` | `http://departamentos-service:8082` |
| cualquier otra | `404` en JSON del propio Gateway (no enrutada) |

Tecnología: **Gateway de aplicación** en Node.js + Express + `http-proxy-middleware` (sección 1.3), no un enrutador declarativo (Nginx/Traefik), porque el Reto 5 exige validar JWT, el Reto 10 validar JWKS y propagar identidad, y el proyecto final componer respuestas: todo eso requiere código. Código: `microservicios/api-gateway/index.js`.

### 5.2 Parámetros del Circuit Breaker (sección 2.3)

Librería `opossum` (sección 2.5), en quien llama: `microservicios/gestion-empleados/src/clients/departamentos.client.js`.

| Parámetro | Valor | Sugerido por el PDF |
|---|---|---|
| Timeout de llamada | 5 s por intento + 3 reintentos (200 ms entre cada uno) | 5 s (Reto 2) |
| Umbral de fallos | 50 % con mínimo 4 llamadas (`volumeThreshold`) en una ventana de 60 s | 3-5 fallos o 50 % de una ventana |
| Timeout del circuito (`resetTimeout`) | 30 s | 30-60 s |

### 5.3 Fallback y reconciliación (sección 2.6)

- **Opción elegida:** registrar con validación pendiente (la segunda de la tabla del PDF) → `201` con `validacionDepartamento: "PENDIENTE"`.
- **Disponibilidad vs. consistencia:** se prioriza **disponibilidad** — RR. HH. sigue registrando aunque departamentos esté caído — aceptando consistencia eventual, porque la reconciliación es automática y visible.
- **Campo aparte, no `estado`:** el pseudocódigo del PDF usa `estado: "PENDIENTE_VALIDACION"`, pero `estado` ya es el ciclo laboral (`ACTIVO` / `EN_VACACIONES` / `RETIRADO`, Reto 1); mezclarlos impediría representar "de vacaciones y pendiente de validar". Por eso `validacion_departamento` es una columna independiente: `PENDIENTE` / `ACEPTADO` / `RECHAZADO`.
- **Reconciliación:** el evento `close` del circuito (`opossum`) dispara `reconciliarPendientes()` (`server.js` → `services/empleados.service.js`), que vuelve a consultar cada pendiente. Sin jobs, sin webhooks, sin intervención manual.
- **Descartadas:** rechazar con `503` (bloquea a RR. HH. durante toda la caída) y departamento por defecto (el PDF: "nunca haga esto: inventa datos").

## 6. Colección Bruno y pruebas automatizadas

```bash
# Colección (con el sistema levantado): 10 peticiones, 13 tests, URL base http://localhost:8080
cd docs/reto-03/bruno && npx @usebruno/cli run --env Local

# Unitarias y de API de empleados (no requieren Docker): 34 pruebas
npm test

# Integración de departamentos, vía Gateway (requieren el sistema levantado y PHP): 17 verificaciones
php microservicios/gestion-departamentos/test/test_departamentos.php
```

También se abre desde la app de Bruno: *Open Collection* → `docs/reto-03/bruno`, entorno `Local`.

## 7. Preguntas probables en la sustentación

| Pregunta | Respuesta corta | Dónde mostrarlo |
|---|---|---|
| ¿Cómo sé que no se puede entrar directo a un servicio? | Usan `expose:`, no `ports:`; `curl :8081`/`:8082` → conexión rechazada | §4.2, `docker compose ps` |
| ¿El Gateway altera las respuestas? | No: pasa el código, las cabeceras (`x-powered-by: PHP`, `Location`) y el cuerpo del servicio | §4.1 |
| ¿Qué pasa si un servicio se cae? | El Gateway responde `503` JSON; su `/health` dice cuál está `DOWN` | §4.3, §3 |
| ¿Cómo sé en qué estado está el circuito? | `GET /empleados/circuito-departamentos` o `GET /health`; también los logs | §3 |
| ¿Por qué abre tras 4 fallos? | `volumeThreshold: 4` + 50 % (el PDF sugiere 3-5 fallos) | §5.2 |
| ¿Cómo se recupera sin reiniciar? | Tras 30 s pasa a `HALF_OPEN`; la siguiente petición real prueba la dependencia y, si responde, cierra | §4.4 pasos 4-5 |
| ¿Por qué el paso 5 da `400` y no el fallback? | Porque consultó de verdad: un `400` de departamento inexistente solo puede venir de departamentos | §4.4 paso 5 |
| ¿Disponibilidad o consistencia? | Disponibilidad, con consistencia eventual y reconciliación automática | §5.3 |
| ¿Qué pasa con un pendiente cuyo departamento no existía? | Al reconciliar queda `RECHAZADO` | §4.4 paso 6 |
| ¿Por qué no usaron `estado: PENDIENTE_VALIDACION`? | `estado` ya es el ciclo laboral; se usó un campo independiente | §5.3 |
| ¿Por qué el curl del PDF da `400`? | El cuerpo reducido no cumple el modelo canónico del Reto 1; falla la validación antes del Circuit Breaker | §4.4 |
| ¿El Gateway tiene lógica de negocio? | No: solo enruta y reporta salud; las reglas viven en los servicios | `api-gateway/index.js` |
| ¿Se rompió la compatibilidad con el Reto 2? | No: las rutas internas son las mismas, solo dejaron de ser accesibles desde fuera | §5.1 |
| ¿Por qué un circuito `OPEN` no marca a empleados como `DOWN`? | Porque sigue atendiendo con el fallback: degradado, no caído | §3 |

## 8. Problemas comunes

| Síntoma | Causa y solución |
|---|---|
| En §4.4 el circuito abre tras **3** peticiones lentas (o menos), no 4 | El umbral es "50 % de fallos con un mínimo de 4 llamadas **en los últimos 60 s**": si justo antes hubo registros exitosos (otra prueba, Bruno…), cuentan dentro de la ventana y el mínimo se alcanza antes. Es el comportamiento correcto; para la demostración, esperar **60 s sin tráfico** antes del paso 3 (o `docker compose restart empleados-service`) y el salto vuelve a ser exactamente 4 lentas → 5ª instantánea |
| `column "validacion_departamento" does not exist` en los logs de empleados | Volumen de una versión anterior: `docker compose down -v` y volver a levantar |
| `Bind for 0.0.0.0:8080 failed: port is already allocated` | Otro proceso usa el 8080: detenerlo |
| Todas las peticiones de §4.4 responden `400` al instante | Se envió el cuerpo reducido del PDF: usar el modelo completo |
| Tras `sleep 35` el circuito sigue en `HALF_OPEN` | Es lo esperado: pasa a `HALF_OPEN` solo por tiempo (`resetTimeout`), pero necesita **una petición real** (paso 5) para probar la dependencia y pasar a `CLOSED` |
| `POST /departamentos` de IT responde `400` | Ya existía de una corrida anterior (los datos persisten en el volumen); se puede seguir |
| `docker compose ps` sigue `healthy` justo después de tumbar una BD | Docker necesita ~25 s de chequeos fallidos; el `/health` del Gateway ya lo muestra |
