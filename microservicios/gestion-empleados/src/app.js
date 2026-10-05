const express = require("express");
const swaggerUi = require("swagger-ui-express");

const { AppError } = require("./errores");
const { crearRouterEmpleados } = require("./routes/empleados.routes");
const { openapiSpec } = require("./openapi");

const FRASES_ESTADO = {
  400: "Bad Request",
  404: "Not Found",
  409: "Conflict",
  500: "Internal Server Error",
  503: "Service Unavailable"
};

const TIMEOUT_HEALTH_DB_MS = 2000;

function crearApp(
  servicioEmpleados,
  clienteDepartamentos = { estadoActual: () => "DESCONOCIDO" },
  { verificarBaseDeDatos = async () => {}, publicador = { estadoActual: () => "DESCONOCIDO" } } = {}
) {
  const app = express();

  app.use(express.json());

  // Documentación bajo el prefijo del servicio: el Gateway solo enruta /empleados/*.
  // Van ANTES del router: si no, GET /empleados/:id tomaría "docs" como un id.
  app.use("/empleados/docs", swaggerUi.serve, swaggerUi.setup(openapiSpec));
  app.get("/empleados/openapi.json", (req, res) => res.json(openapiSpec));

  // Salud interna del servicio (la usa el healthcheck de docker-compose y el /health
  // agregado del Gateway; no se enruta hacia fuera). El servicio está DOWN solo si su
  // propia base de datos no responde: un circuito OPEN hacia departamentos significa
  // "degradado pero vivo" (sigue registrando con fallback PENDIENTE), no caído. Lo
  // mismo con el broker: sin él se sigue registrando, solo fallan las publicaciones.
  app.get("/health", async (req, res) => {
    const db = (await responde(verificarBaseDeDatos, TIMEOUT_HEALTH_DB_MS)) ? "UP" : "DOWN";
    const status = db === "UP" ? "UP" : "DOWN";
    res.status(status === "UP" ? 200 : 503).json({
      status,
      service: "empleados-service",
      timestamp: new Date().toISOString(),
      components: {
        app: "UP",
        db,
        circuitoDepartamentos: clienteDepartamentos.estadoActual(),
        broker: publicador.estadoActual()
      }
    });
  });

  // Bajo /empleados/* a propósito: así queda alcanzable a través del Gateway sin
  // agregar una ruta nueva al enrutamiento del Reto 3 (que enruta exactamente
  // /empleados/* y /departamentos/*, nada más). Debe ir ANTES del router de
  // empleados para no chocar con GET /empleados/:id.
  app.get("/empleados/circuito-departamentos", (req, res) => {
    res.status(200).json({
      dependencia: "departamentos-service",
      estado: clienteDepartamentos.estadoActual()
    });
  });

  app.use(crearRouterEmpleados(servicioEmpleados));

  app.use((req, res) => {
    res.status(404).json(construirCuerpoError(404, "Recurso no encontrado", req));
  });

  app.use((error, req, res, next) => {
    manejarError(error, req, res);
  });

  return app;
}

// true si la verificación termina sin error dentro del plazo; false si falla o tarda más
// (un pool de pg con la BD caída puede quedarse esperando la conexión indefinidamente).
async function responde(verificacion, timeoutMs) {
  let timer;
  const plazo = new Promise((_, reject) => {
    timer = setTimeout(() => reject(new Error("timeout")), timeoutMs);
  });
  try {
    await Promise.race([verificacion(), plazo]);
    return true;
  } catch {
    return false;
  } finally {
    clearTimeout(timer);
  }
}

function construirCuerpoError(status, mensaje, req, errores = []) {
  const cuerpo = {
    status,
    error: FRASES_ESTADO[status] ?? "Error",
    message: mensaje,
    timestamp: new Date().toISOString(),
    path: req.originalUrl
  };
  if (errores.length > 0) {
    cuerpo.errors = errores;
  }
  return cuerpo;
}

function manejarError(error, req, res) {
  if (error instanceof AppError) {
    return res
      .status(error.codigoEstado)
      .json(construirCuerpoError(error.codigoEstado, error.message, req, error.errores));
  }

  if (error.type === "entity.parse.failed") {
    return res.status(400).json(construirCuerpoError(400, "Cuerpo JSON inválido", req));
  }

  console.error(error);
  return res.status(500).json(construirCuerpoError(500, "Error interno del servidor", req));
}

module.exports = { crearApp };