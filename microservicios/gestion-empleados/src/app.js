const express = require("express");
const swaggerUi = require("swagger-ui-express");

const { AppError } = require("./errores");
const { crearRouterEmpleados } = require("./routes/empleados.routes");
const { openapiSpec } = require("./openapi");

const FRASES_ESTADO = {
  400: "Bad Request",
  404: "Not Found",
  500: "Internal Server Error",
  503: "Service Unavailable"
};

function crearApp(servicioEmpleados) {
  const app = express();

  app.use(express.json());

  app.use("/docs", swaggerUi.serve, swaggerUi.setup(openapiSpec));
  app.get("/openapi.json", (req, res) => res.json(openapiSpec));

  app.use(crearRouterEmpleados(servicioEmpleados));

  app.use((req, res) => {
    res.status(404).json(construirCuerpoError(404, "Recurso no encontrado", req));
  });

  app.use((error, req, res, next) => {
    manejarError(error, req, res);
  });

  return app;
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