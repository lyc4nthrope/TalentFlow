const { MOTIVOS_RETIRO } = require("./dominio/empleado");

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
    },
    fechaRetiro: {
      type: "string",
      format: "date-time",
      nullable: true,
      readOnly: true,
      description: "Instante (UTC) de la baja lógica. null mientras el empleado no esté RETIRADO.",
      example: "2026-11-30T16:45:00Z"
    },
    motivoRetiro: {
      type: "string",
      enum: MOTIVOS_RETIRO,
      nullable: true,
      readOnly: true,
      example: "RENUNCIA"
    }
  }
};

const actualizacionSchema = {
  type: "object",
  description:
    "Campos editables: exactamente los que replica el evento empleado.actualizado. Los de solo lectura (id, numeroEmpleado, fechaIngreso, estado...) se aceptan solo si no cambian; un campo desconocido responde 400.",
  required: ["nombre", "apellido", "email", "cargo", "area", "departamentoId"],
  properties: {
    nombre: { type: "string", example: "Juan" },
    apellido: { type: "string", example: "Pérez Gómez" },
    email: { type: "string", format: "email", example: "juan.perez@empresa.com" },
    cargo: { type: "string", example: "Tech Lead" },
    area: { type: "string", example: "Tecnología" },
    departamentoId: { type: "string", example: "IT" }
  }
};

const respuestaError = (descripcion) => ({
  description: descripcion,
  content: { "application/json": { schema: { $ref: "#/components/schemas/Error" } } }
});

const parametroId = { name: "id", in: "path", required: true, schema: { type: "string" }, example: "E001" };

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
    version: "0.3.0",
    description:
      "Registro, actualización, baja lógica y auditoría de empleados. Persiste en PostgreSQL, valida el departamento contra departamentos-service y publica los eventos empleado.creado, empleado.actualizado y empleado.retirado (Catálogo de Eventos)."
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
              "Empleado registrado y evento empleado.creado publicado. Si departamentos-service no respondió (Circuit Breaker abierto), se registra igual con validacionDepartamento=PENDIENTE y se reconcilia automáticamente cuando el servicio se restablece.",
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
        summary: "Lista empleados, con filtros de auditoría",
        description:
          "Sin filtros lista todos. Auditoría de retiros: ?estado=RETIRADO&desde=YYYY-MM-DD&hasta=YYYY-MM-DD (rango inclusivo sobre el día del retiro en America/Bogota). desde/hasta exigen estado=RETIRADO.",
        parameters: [
          { name: "estado", in: "query", schema: { type: "string", enum: ["ACTIVO", "EN_VACACIONES", "RETIRADO"] } },
          { name: "desde", in: "query", schema: { type: "string", format: "date" }, example: "2026-01-01" },
          { name: "hasta", in: "query", schema: { type: "string", format: "date" }, example: "2026-06-30" }
        ],
        responses: {
          200: {
            description: "Listado de empleados",
            content: {
              "application/json": {
                schema: { type: "array", items: { $ref: "#/components/schemas/Empleado" } }
              }
            }
          },
          400: respuestaError("Filtros inválidos (estado desconocido, fecha mal formada, desde > hasta o desde/hasta sin estado=RETIRADO)")
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
    "/health": {
      get: {
        summary: "Salud interna del servicio (no enrutada por el Gateway)",
        responses: {
          200: { description: "Servicio y base de datos UP", content: { "application/json": { schema: { $ref: "#/components/schemas/Health" } } } },
          503: { description: "La base de datos no responde", content: { "application/json": { schema: { $ref: "#/components/schemas/Health" } } } }
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
      },
      put: {
        summary: "Actualiza un empleado y publica empleado.actualizado",
        parameters: [parametroId],
        requestBody: {
          required: true,
          content: { "application/json": { schema: { $ref: "#/components/schemas/ActualizacionEmpleado" } } }
        },
        responses: {
          200: {
            description: "Empleado actualizado",
            content: { "application/json": { schema: { $ref: "#/components/schemas/Empleado" } } }
          },
          400: respuestaError("Campo faltante, desconocido o de solo lectura modificado; email inválido o ya usado por otro empleado; departamento inexistente"),
          404: respuestaError("El empleado no existe"),
          409: respuestaError("El empleado está RETIRADO y no se puede modificar")
        }
      },
      delete: {
        summary: "Retira a un empleado (baja lógica) y publica empleado.retirado",
        description:
          "No borra el registro: cambia el estado a RETIRADO y guarda fechaRetiro y motivoRetiro, que quedan disponibles para auditoría.",
        parameters: [parametroId],
        requestBody: {
          required: false,
          content: {
            "application/json": {
              schema: {
                type: "object",
                properties: {
                  motivo: { type: "string", enum: MOTIVOS_RETIRO, default: "RENUNCIA" }
                }
              }
            }
          }
        },
        responses: {
          200: {
            description: "Empleado retirado (estado RETIRADO, con fechaRetiro y motivoRetiro)",
            content: { "application/json": { schema: { $ref: "#/components/schemas/Empleado" } } }
          },
          400: respuestaError("Motivo de retiro no válido"),
          404: respuestaError("El empleado no existe"),
          409: respuestaError("El empleado ya estaba RETIRADO")
        }
      }
    }
  },
  components: {
    schemas: {
      Empleado: empleadoSchema,
      ActualizacionEmpleado: actualizacionSchema,
      Error: errorSchema,
      Health: {
        type: "object",
        properties: {
          status: { type: "string", enum: ["UP", "DOWN"] },
          service: { type: "string", example: "empleados-service" },
          timestamp: { type: "string", format: "date-time" },
          components: {
            type: "object",
            properties: {
              app: { type: "string", enum: ["UP"] },
              db: { type: "string", enum: ["UP", "DOWN"] },
              circuitoDepartamentos: { type: "string", enum: ["CLOSED", "OPEN", "HALF_OPEN"] },
              broker: { type: "string", enum: ["UP", "DOWN"], description: "Conexión con RabbitMQ para publicar eventos" }
            }
          }
        }
      }
    }
  }
};

module.exports = { openapiSpec };