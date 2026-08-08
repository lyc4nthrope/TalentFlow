const express = require("express");

const { errores } = require("@talentflow/shared");

const { AppError } = errores;

function crearApp(servicioEmpleados) {
  const app = express();

  app.use(express.json());

  app.post("/empleados", (req, res) => {
    const empleado = servicioEmpleados.registrar(req.body ?? {});
    res.status(200).json(empleado);
  });

  app.get("/empleados/:id", (req, res) => {
    const empleado = servicioEmpleados.consultarPorId(req.params.id);
    res.status(200).json(empleado);
  });

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
