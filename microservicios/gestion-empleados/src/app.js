const express = require("express");
const swaggerUi = require("swagger-ui-express");

const { errores } = require("@talentflow/shared");
const { crearRouterEmpleados } = require("./routes/empleados.routes");
const { openapiSpec } = require("./openapi");

const { AppError } = errores;

function crearApp(servicioEmpleados) {
  const app = express();

  app.use(express.json());

  app.use("/docs", swaggerUi.serve, swaggerUi.setup(openapiSpec));
  app.get("/openapi.json", (req, res) => res.json(openapiSpec));

  app.use(crearRouterEmpleados(servicioEmpleados));

  app.use((req, res) => {
    res.status(404).json({ error: "Recurso no encontrado" });
  });

  app.use((error, req, res, next) => {
    manejarError(error, res);
  });

  return app;
}

function manejarError(error, res) {
  if (error instanceof AppError) {
    return res.status(error.codigoEstado).json({ error: error.message });
  }

  if (error.type === "entity.parse.failed") {
    return res.status(400).json({ error: "Cuerpo JSON inválido" });
  }

  console.error(error);
  return res.status(500).json({ error: "Error interno del servidor" });
}

module.exports = { crearApp };