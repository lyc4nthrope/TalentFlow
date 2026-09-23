const { crearApp } = require("./app");
const { crearServicioEmpleados } = require("./services/empleados.service");
const { crearRepositorioEmpleadosPostgres } = require("./repository/empleados.repository.postgres");
const { crearClienteDepartamentos } = require("./clients/departamentos.client");
const { crearPool } = require("./db/pool");

const PUERTO = process.env.PORT || 8080;
const DEPARTAMENTOS_SERVICE_URL =
  process.env.DEPARTAMENTOS_SERVICE_URL || "http://localhost:8081";

const pool = crearPool();
const repositorio = crearRepositorioEmpleadosPostgres(pool);
const clienteDepartamentos = crearClienteDepartamentos({
  baseUrl: DEPARTAMENTOS_SERVICE_URL,
  timeoutMs: process.env.DEPARTAMENTOS_TIMEOUT_MS
    ? Number(process.env.DEPARTAMENTOS_TIMEOUT_MS)
    : 5000,
  maxReintentos: process.env.DEPARTAMENTOS_MAX_REINTENTOS
    ? Number(process.env.DEPARTAMENTOS_MAX_REINTENTOS)
    : 3
});
const servicio = crearServicioEmpleados(repositorio, clienteDepartamentos);
const app = crearApp(servicio, clienteDepartamentos);

// Cuando el Circuit Breaker vuelve a CERRAR (departamentos-service se restableció),
// revisa automáticamente a los empleados que quedaron PENDIENTE y los mueve a
// ACEPTADO o RECHAZADO según la respuesta real del servicio.
clienteDepartamentos.onRecuperado(() => {
  servicio
    .reconciliarPendientes()
    .then((resultados) => {
      if (resultados.length > 0) {
        console.info(`🔁 Reconciliados ${resultados.length} empleado(s) pendiente(s):`, resultados);
      }
    })
    .catch((error) => console.error("Error reconciliando empleados pendientes:", error));
});

app.listen(PUERTO, () => {
  console.log(`Servicio de empleados escuchando en http://localhost:${PUERTO}`);
  console.log(`Documentación Swagger en http://localhost:${PUERTO}/docs`);
});