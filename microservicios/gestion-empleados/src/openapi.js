const empleadoSchema = {
  type: "object",
  properties: {
    id: { type: "string", example: "E001" },
    nombre: { type: "string", example: "Juan" },
    apellido: { type: "string", example: "Pérez" },
    email: { type: "string", format: "email", example: "juan.perez@empresa.com" },
    numeroEmpleado: { type: "string", example: "EMP-2026-001" },
    cargo: { type: "string", example: "Desarrollador Senior" },
    area: { type: "string", example: "Tecnología" },
    departamentoId: { type: "string", example: "IT" },
    fechaIngreso: { type: "string", format: "date", example: "2026-02-10" },
    estado: {
      type: "string",
      enum: ["ACTIVO", "EN_VACACIONES", "RETIRADO"],
      example: "ACTIVO"
    }
  }
};

const errorSchema = {
  type: "object",
  properties: {
    status: { type: "integer", example: 400 },
    error: { type: "string", example: "Bad Request" },
    message: { type: "string", example: "El email juan.perez@empresa.com ya está registrado" },
    timestamp: { type: "string", format: "date-time" },
    path: { type: "string", example: "/empleados" },
    errors: {
      type: "array",
      items: {
        type: "object",
        properties: {
          field: { type: "string" },
          message: { type: "string" },
          rejectedValue: {}
        }
      }
    }
  }
};

const openapiSpec = {
  openapi: "3.0.3",
  info: {
    title: "TalentFlow - Servicio de Gestión de Empleados",
    version: "0.2.0",
    description:
      "Registro y consulta de empleados. Persiste en PostgreSQL y valida el departamento contra el servicio de departamentos."
  },
  servers: [{ url: "/" }],
  paths: {
    "/empleados": {
      post: {
        summary: "Registra un nuevo empleado",
        requestBody: {
          required: true,
          content: {
            "application/json": {
              schema: {
                allOf: [
                  { $ref: "#/components/schemas/Empleado" },
                  { required: ["id", "nombre", "apellido", "email", "numeroEmpleado", "cargo", "area", "departamentoId", "fechaIngreso"] }
                ]
              }
            }
          }
        },
        responses: {
          201: {
            description: "Empleado registrado",
            headers: {
              Location: {
                description: "URL del empleado creado",
                schema: { type: "string", example: "/empleados/E001" }
              }
            },
            content: { "application/json": { schema: { $ref: "#/components/schemas/Empleado" } } }
          },
          400: {
            description:
              "Email duplicado, numeroEmpleado duplicado, campos faltantes o departamento inexistente",
            content: { "application/json": { schema: { $ref: "#/components/schemas/Error" } } }
          },
          503: {
            description: "El servicio de departamentos no respondió tras los reintentos",
            content: { "application/json": { schema: { $ref: "#/components/schemas/Error" } } }
          }
        }
      },
      get: {
        summary: "Lista todos los empleados registrados",
        responses: {
          200: {
            description: "Listado de empleados",
            content: {
              "application/json": {
                schema: { type: "array", items: { $ref: "#/components/schemas/Empleado" } }
              }
            }
          }
        }
      }
    },
    "/empleados/{id}": {
      get: {
        summary: "Consulta un empleado por id",
        parameters: [
          { name: "id", in: "path", required: true, schema: { type: "string" }, example: "E001" }
        ],
        responses: {
          200: {
            description: "Empleado encontrado",
            content: { "application/json": { schema: { $ref: "#/components/schemas/Empleado" } } }
          },
          404: {
            description: "El empleado no existe",
            content: { "application/json": { schema: { $ref: "#/components/schemas/Error" } } }
          }
        }
      }
    }
  },
  components: {
    schemas: {
      Empleado: empleadoSchema,
      Error: errorSchema
    }
  }
};

module.exports = { openapiSpec };