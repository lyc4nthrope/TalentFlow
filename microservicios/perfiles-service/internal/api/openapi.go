package api

// especOpenAPI es la especificación OpenAPI 3 del servicio. Se sirve en
// /perfiles/openapi.json y la consume Swagger UI en /perfiles/docs.
const especOpenAPI = `{
  "openapi": "3.0.3",
  "info": {
    "title": "TalentFlow — Servicio de Perfiles",
    "version": "1.0.0",
    "description": "Consulta y actualiza perfiles de empleados. Los perfiles se crean, sincronizan y archivan automáticamente al consumir empleado.creado, empleado.actualizado y empleado.retirado (ver Catálogo de Eventos). Los campos replicados (nombre, apellido, email, cargo, area, departamentoId) son de solo lectura aquí: su dueño es empleados-service."
  },
  "servers": [{ "url": "/", "description": "API Gateway" }],
  "tags": [{ "name": "Perfiles" }],
  "paths": {
    "/perfiles": {
      "get": {
        "tags": ["Perfiles"],
        "summary": "Lista todos los perfiles",
        "parameters": [{
          "name": "archivado", "in": "query", "required": false,
          "description": "Filtra por estado de archivo. Sin este parámetro se listan todos.",
          "schema": { "type": "boolean" }
        }],
        "responses": {
          "200": { "description": "Lista de perfiles", "content": { "application/json": { "schema": { "type": "array", "items": { "$ref": "#/components/schemas/Perfil" } } } } },
          "400": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/perfiles/{empleadoId}": {
      "parameters": [{ "name": "empleadoId", "in": "path", "required": true, "schema": { "type": "string" }, "example": "E001" }],
      "get": {
        "tags": ["Perfiles"],
        "summary": "Consulta el perfil de un empleado",
        "responses": {
          "200": { "description": "El perfil", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Perfil" } } } },
          "404": { "$ref": "#/components/responses/Error" }
        }
      },
      "put": {
        "tags": ["Perfiles"],
        "summary": "Actualiza el perfil (teléfono, dirección, ciudad, biografía)",
        "description": "Actualización parcial: solo cambian los campos enviados. Enviar un campo replicado (nombre, email, ...) o desconocido responde 400. Un perfil archivado responde 409.",
        "requestBody": {
          "required": true,
          "content": { "application/json": { "schema": { "$ref": "#/components/schemas/ActualizarPerfil" } } }
        },
        "responses": {
          "200": { "description": "Perfil actualizado", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Perfil" } } } },
          "400": { "$ref": "#/components/responses/Error" },
          "404": { "$ref": "#/components/responses/Error" },
          "409": { "$ref": "#/components/responses/Error" }
        }
      }
    }
  },
  "components": {
    "responses": {
      "Error": { "description": "Error", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Error" } } } }
    },
    "schemas": {
      "Perfil": {
        "type": "object",
        "properties": {
          "id": { "type": "string", "format": "uuid" },
          "empleadoId": { "type": "string", "example": "E001" },
          "nombre": { "type": "string", "example": "Juan", "readOnly": true },
          "apellido": { "type": "string", "example": "Pérez", "readOnly": true },
          "email": { "type": "string", "example": "juan.perez@empresa.com", "readOnly": true },
          "cargo": { "type": "string", "example": "Desarrollador Senior", "readOnly": true },
          "area": { "type": "string", "example": "Tecnología", "readOnly": true },
          "departamentoId": { "type": "string", "example": "IT", "readOnly": true },
          "telefono": { "type": "string", "example": "3001234567" },
          "direccion": { "type": "string", "example": "Calle 10 # 5-20" },
          "ciudad": { "type": "string", "example": "Armenia" },
          "biografia": { "type": "string", "example": "Ingeniero de sistemas." },
          "archivado": { "type": "boolean", "description": "true tras empleado.retirado. El perfil nunca se borra." },
          "fechaArchivado": { "type": "string", "format": "date-time", "nullable": true },
          "fechaCreacion": { "type": "string", "format": "date-time" },
          "fechaActualizacion": { "type": "string", "format": "date-time" }
        }
      },
      "ActualizarPerfil": {
        "type": "object",
        "properties": {
          "telefono": { "type": "string", "maxLength": 30, "example": "3001234567" },
          "direccion": { "type": "string", "maxLength": 200, "example": "Calle 10 # 5-20" },
          "ciudad": { "type": "string", "maxLength": 100, "example": "Armenia" },
          "biografia": { "type": "string", "maxLength": 2000, "example": "Ingeniero de sistemas con 8 años de experiencia." }
        }
      },
      "Error": {
        "type": "object",
        "properties": {
          "status": { "type": "integer", "example": 404 },
          "error": { "type": "string", "example": "Not Found" },
          "message": { "type": "string", "example": "No existe un perfil para el empleado E999" },
          "timestamp": { "type": "string", "format": "date-time" },
          "path": { "type": "string", "example": "/perfiles/E999" },
          "errors": {
            "type": "array",
            "items": { "type": "object", "properties": { "campo": { "type": "string" }, "mensaje": { "type": "string" } } }
          }
        }
      }
    }
  }
}`

const paginaSwagger = `<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <title>TalentFlow — Perfiles (Swagger UI)</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({ url: "/perfiles/openapi.json", dom_id: "#swagger-ui" });
  </script>
</body>
</html>`
