# Reto 3 — Guion para capturas / video

El reto exige evidencias (capturas o video) de tres cosas. Este guion las cubre en un solo video de ~3-4 minutos, en el orden de la sección 3 de `reto3.pdf`.

## Preparación (antes de grabar)

```bash
docker compose down -v
docker compose up --build -d
docker compose ps        # esperar a que departamentos-service diga (healthy)
```

Dos terminales visibles: **A** para los comandos y **B** con los logs del Circuit Breaker:

```bash
# Terminal B
docker compose logs -f empleados-service
```

Opción rápida: correr `bash docs/reto-03/demo.sh` en la terminal A y narrar sobre su salida. Opción manual (más didáctica): los pasos de abajo.

## Evidencia 1 — Acceso directo rechazado vs. Gateway (≈40 s)

```bash
docker compose ps                                  # mostrar: solo api-gateway tiene 0.0.0.0:8080
curl -i http://localhost:8081/departamentos        # conexión rechazada
curl -i http://localhost:8082/empleados            # conexión rechazada
curl -i http://localhost:8080/departamentos        # 200 por el Gateway
curl http://localhost:8080/health                  # Gateway UP + estado de cada servicio y del circuito (sistema: OK)
```

**Decir:** "Solo el Gateway publica puerto; los servicios usan `expose`. El mismo recurso falla directo y funciona por la URL base `http://localhost:8080`."

Extra (sección 3.2):

```bash
docker compose stop departamentos-service
curl -i http://localhost:8080/departamentos        # 503 con JSON, no página del framework
docker compose start departamentos-service
```

## Evidencia 2 — Salto en el tiempo de respuesta (≈60 s)

> Antes de este paso, **60 s sin hacer peticiones** (o `docker compose restart empleados-service`): el Circuit Breaker cuenta las llamadas de los últimos 60 s, y los registros exitosos recientes harían que abra antes de la 4ª petición.

```bash
curl -X POST http://localhost:8080/departamentos -H "Content-Type: application/json" \
  -d '{"id": "IT", "nombre": "Tecnología"}'
curl http://localhost:8080/empleados/circuito-departamentos     # CLOSED

docker compose stop departamentos-service

for i in 1 2 3 4 5 6 7 8; do
  curl -s -o /dev/null -w "Petición $i -> HTTP %{http_code} | %{time_total}s\n" \
    -X POST http://localhost:8080/empleados -H "Content-Type: application/json" \
    -d "{\"id\":\"V00$i\",\"nombre\":\"Test\",\"apellido\":\"Video\",\"email\":\"v$i@video.test\",\"numeroEmpleado\":\"V$i\",\"cargo\":\"Dev\",\"area\":\"IT\",\"departamentoId\":\"IT\",\"fechaIngreso\":\"2026-01-01\"}"
done
curl http://localhost:8080/empleados/circuito-departamentos     # OPEN
curl http://localhost:8080/empleados/V001                       # validacionDepartamento: PENDIENTE
curl http://localhost:8080/health    # sistema: DEGRADADO, departamentos-service DOWN, circuito OPEN
```

**Señalar:** peticiones 1-4 ≈ 0.6 s, desde la 5ª ≈ 0.01 s; en la terminal B aparece `⚠️ Circuit Breaker ABIERTO`.

**Decir:** "Tras 4 fallos el circuito abre y deja de llamar a un servicio que ya sabe caído. El fallback es de negocio: el empleado se registra como `PENDIENTE` (disponibilidad sobre consistencia) en vez de rechazarse."

## Evidencia 3 — Recuperación automática (≈60 s)

```bash
docker compose start departamentos-service
sleep 35
curl http://localhost:8080/empleados/circuito-departamentos     # HALF_OPEN

curl -i -X POST http://localhost:8080/empleados -H "Content-Type: application/json" \
  -d '{"id":"V100","nombre":"Recuperado","apellido":"Video","email":"v100@video.test","numeroEmpleado":"V100","cargo":"Dev","area":"IT","departamentoId":"NO-EXISTE","fechaIngreso":"2026-01-01"}'
                                                                # 400: consultó de verdad
curl http://localhost:8080/empleados/circuito-departamentos     # CLOSED
curl http://localhost:8080/empleados/V001                       # validacionDepartamento: ACEPTADO
curl http://localhost:8080/health    # sistema: OK de nuevo
```

**Señalar:** en la terminal B, `🔄 HALF-OPEN` → `✅ CERRADO` → `🔁 Reconciliados 8 empleado(s)`.

**Decir:** "Nadie reinició nada. El `400` solo puede venir de una consulta real, así que el circuito cerró; y al cerrar, los pendientes se reconciliaron solos a `ACEPTADO` o `RECHAZADO`."

## Si el profesor usa el guion literal del PDF

El PDF envía `{"id","nombre","departamentoId"}`. Aquí responde `400` inmediato por campos faltantes (modelo canónico del Reto 1), antes de llegar al Circuit Breaker. Explicarlo y usar los cuerpos completos de arriba.
