const { Router } = require("express");

function crearRouterEmpleados(servicioEmpleados) {
  const router = Router();

  router.post("/empleados", async (req, res, next) => {
    try {
      const empleado = await servicioEmpleados.registrar(req.body ?? {});
      res.status(201).json(empleado);
    } catch (error) {
      next(error);
    }
  });

  router.get("/empleados", async (req, res, next) => {
    try {
      const empleados = await servicioEmpleados.listar();
      res.status(200).json(empleados);
    } catch (error) {
      next(error);
    }
  });

  router.get("/empleados/:id", async (req, res, next) => {
    try {
      const empleado = await servicioEmpleados.consultarPorId(req.params.id);
      res.status(200).json(empleado);
    } catch (error) {
      next(error);
    }
  });

  return router;
}

module.exports = { crearRouterEmpleados };