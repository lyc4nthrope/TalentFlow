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
    },
    validacionDepartamento: {
      type: "string",
      enum: ["PENDIENTE", "ACEPTADO", "RECHAZADO"],
      description:
        "PENDIENTE si se registró con departamentos-service caído (Circuit Breaker abierto); se reconcilia solo a ACEPTADO o RECHAZADO cuando el servicio se restablece.",
      example: "ACEPTADO"
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
            description:
              "Empleado registrado. Si departamentos-service no respondió (Circuit Breaker abierto), se registra igual con validacionDepartamento=PENDIENTE y se reconcilia automáticamente cuando el servicio se restablece.",
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
              "Email duplicado, numeroEmpleado duplicado, campos faltantes o departamento inexistente (verificado con departamentos-service disponible)",
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
    "/empleados/circuito-departamentos": {
      get: {
        summary: "Estado del Circuit Breaker hacia departamentos-service",
        responses: {
          200: {
            description: "Estado actual del circuito",
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  properties: {
                    dependencia: { type: "string", example: "departamentos-service" },
                    estado: { type: "string", enum: ["CLOSED", "OPEN", "HALF_OPEN"], example: "CLOSED" }
                  }
                }
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