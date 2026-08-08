# Microservicio de Gestión de Empleados (Reto 1)

Servicio web para la gestión básica de empleados. Primera pieza del sistema de onboarding y offboarding basado en microservicios.

## Endpoints

### Registrar un empleado

```
POST /empleados
```

Cuerpo (modelo canónico):

```json
{
  "id": "E001",
  "nombre": "Juan",
  "apellido": "Pérez",
  "email": "juan.perez@empresa.com",
  "numeroEmpleado": "EMP-2026-001",
  "cargo": "Desarrollador Senior",
  "area": "Tecnología",
  "departamentoId": "IT",
  "fechaIngreso": "2026-02-10"
}
```

- **200 OK**: empleado registrado (el campo `estado` se asigna `ACTIVO` por defecto).
- **400 Bad Request**: email o `numeroEmpleado` ya registrados, campos obligatorios faltantes, email o estado inválidos.

### Consultar un empleado por id

```
GET /empleados/{id}
```

- **200 OK**: información del empleado.
- **404 Not Found**: `El empleado con id {id} no existe`.

### Rutas no soportadas

Cualquier otra ruta o método responde **404** con `Recurso no encontrado`.

## Estructura del código

```
src/
├── repository/empleados.repository.js   # Almacenamiento en memoria (Map)
├── services/empleados.service.js        # Lógica de negocio (reglas y validaciones)
├── app.js                               # Capa HTTP (Express) + manejo de errores
└── server.js                            # Punto de entrada
```

El modelo canónico vive en `@talentflow/shared`.

## Construcción y ejecución (Docker)

Desde la raíz del proyecto:

```bash
docker build -f microservicios/gestion-empleados/Dockerfile -t servidor-empleados .
docker run -p 8080:8080 servidor-empleados
```

## Ejecución local (sin Docker)

Desde la raíz del proyecto:

```bash
npm install
npm run dev:empleados
```

## Pruebas

```bash
npm test
```

## Evidencia

Los resultados de las pruebas se documentan en `docs/reto-01/pruebas.md`.
