const { Router } = require("express");

function crearRouterEmpleados(servicioEmpleados) {
  const router = Router();

  router.post("/empleados", async (req, res, next) => {
    try {
      const empleado = await servicioEmpleados.registrar(req.body ?? {});
      res.status(201).location(`/empleados/${empleado.id}`).json(empleado);
    } catch (error) {
      next(error);
    }
  });

  // Filtros de auditoría (Reto 4): ?estado=RETIRADO&desde=YYYY-MM-DD&hasta=YYYY-MM-DD.
  // Solo se pasan los filtros conocidos; su validación es del dominio.
  router.get("/empleados", async (req, res, next) => {
    try {
      const { estado, desde, hasta } = req.query;
      const empleados = await servicioEmpleados.listar({ estado, desde, hasta });
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

  router.put("/empleados/:id", async (req, res, next) => {
    try {
      const empleado = await servicioEmpleados.actualizar(req.params.id, req.body ?? {});
      res.status(200).json(empleado);
    } catch (error) {
      next(error);
    }
  });

  // Baja lógica: responde 200 con el empleado ya RETIRADO (incluye fechaRetiro).
  // El cuerpo es opcional: {"motivo": "RENUNCIA" | "DESPIDO" | ...}.
  router.delete("/empleados/:id", async (req, res, next) => {
    try {
      const empleado = await servicioEmpleados.retirar(req.params.id, { motivo: req.body?.motivo });
      res.status(200).json(empleado);
    } catch (error) {
      next(error);
    }
  });

  return router;
}

module.exports = { crearRouterEmpleados };
