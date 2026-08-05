# Reto 1 - Evidencia de pruebas

Servicio: `@talentflow/empleados` · Fecha: 2026-08-05

## Resumen

- 19 pruebas automatizadas (Node `node:test`), todas en verde.
- Pruebas unitarias del servicio: 11.
- Pruebas de integración HTTP: 8.
- Verificación manual con `curl` y contenedor Docker.

## Pruebas automatizadas

```bash
npm test
# Resultado: 19 pass, 0 fail
```

### Cobertura de escenarios verificados

| Escenario | Resultado esperado | Estado |
|---|---|---|
| POST /empleados válido | 200 + empleado con `estado: ACTIVO` | ✅ |
| POST email duplicado | 400 + mensaje descriptivo | ✅ |
| POST numeroEmpleado duplicado | 400 + mensaje descriptivo | ✅ |
| POST campos obligatorios faltantes | 400 | ✅ |
| POST email con formato inválido | 400 | ✅ |
| POST estado inválido | 400 | ✅ |
| POST JSON inválido | 400 + `Cuerpo JSON inválido` | ✅ |
| GET /empleados/{id} existente | 200 + empleado | ✅ |
| GET /empleados/{id} inexistente | 404 + `El empleado con id {id} no existe` | ✅ |
| Ruta desconocida | 404 + `Recurso no encontrado` | ✅ |
| Método no soportado | 404 + `Recurso no encontrado` | ✅ |

## Verificación manual con curl

### 1. Registro exitoso

```bash
curl -X POST http://localhost:8080/empleados \
  -H "Content-Type: application/json" \
  -d '{"id":"E001","nombre":"Juan","apellido":"Pérez","email":"juan.perez@empresa.com","numeroEmpleado":"EMP-2026-001","cargo":"Desarrollador Senior","area":"Tecnología","departamentoId":"IT","fechaIngreso":"2026-02-10"}'
```

```json
{"id":"E001","nombre":"Juan","apellido":"Pérez","email":"juan.perez@empresa.com","numeroEmpleado":"EMP-2026-001","cargo":"Desarrollador Senior","area":"Tecnología","departamentoId":"IT","fechaIngreso":"2026-02-10","estado":"ACTIVO"}
```

HTTP **200**.

### 2. Email duplicado

```json
{"error":"El email juan.perez@empresa.com ya está registrado"}
```

HTTP **400**.

### 3. numeroEmpleado duplicado

```json
{"error":"El numeroEmpleado EMP-2026-001 ya está registrado"}
```

HTTP **400**.

### 4. Consulta exitosa

```bash
curl http://localhost:8080/empleados/E001
```

```json
{"id":"E001","nombre":"Juan","apellido":"Pérez","email":"juan.perez@empresa.com","numeroEmpleado":"EMP-2026-001","cargo":"Desarrollador Senior","area":"Tecnología","departamentoId":"IT","fechaIngreso":"2026-02-10","estado":"ACTIVO"}
```

HTTP **200**.

### 5. Empleado no existe

```json
{"error":"El empleado con id E999 no existe"}
```

HTTP **404**.

### 6. Ruta desconocida

```json
{"error":"Recurso no encontrado"}
```

HTTP **404**.

### 7. Método no soportado (DELETE /empleados)

```json
{"error":"Recurso no encontrado"}
```

HTTP **404**.

### 8. JSON inválido

```json
{"error":"Cuerpo JSON inválido"}
```

HTTP **400**.

## Verificación con Docker

```bash
docker build -f services/empleados/Dockerfile -t servidor-empleados .
docker run -p 8080:8080 servidor-empleados
```

Se registró y consultó un empleado dentro del contenedor (`E100`), ambas operaciones respondieron **200**.
