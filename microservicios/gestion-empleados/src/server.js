const { crearApp } = require("./app");
const { crearServicioEmpleados } = require("./services/empleados.service");
const { crearRepositorioEmpleadosPostgres } = require("./repository/empleados.repository.postgres");
const { crearClienteDepartamentos } = require("./clients/departamentos.client");
const { crearPublicadorEventos } = require("./eventos/publicador");
const { crearPool } = require("./db/pool");

const PUERTO = process.env.PORT || 8080;
const DEPARTAMENTOS_SERVICE_URL =
  process.env.DEPARTAMENTOS_SERVICE_URL || "http://localhost:8081";
const ZONA_HORARIA = process.env.ZONA_HORARIA || "America/Bogota";

// Credenciales del broker por variables de entorno (nunca en el código). Se codifican
// para la URL: una contraseña con "@" o ":" rompería una URL armada a mano.
const RABBITMQ_URL =
  `amqp://${encodeURIComponent(process.env.RABBITMQ_USER || "guest")}` +
  `:${encodeURIComponent(process.env.RABBITMQ_PASS || "guest")}` +
  `@${process.env.RABBITMQ_HOST || "localhost"}:${process.env.RABBITMQ_PORT || 5672}`;

const pool = crearPool();
const repositorio = crearRepositorioEmpleadosPostgres(pool, { zonaHoraria: ZONA_HORARIA });
const clienteDepartamentos = crearClienteDepartamentos({
  baseUrl: DEPARTAMENTOS_SERVICE_URL,
  timeoutMs: process.env.DEPARTAMENTOS_TIMEOUT_MS
    ? Number(process.env.DEPARTAMENTOS_TIMEOUT_MS)
    : 5000,
  maxReintentos: process.env.DEPARTAMENTOS_MAX_REINTENTOS
    ? Number(process.env.DEPARTAMENTOS_MAX_REINTENTOS)
    : 3
});
const publicador = crearPublicadorEventos({
  url: RABBITMQ_URL,
  exchange: process.env.RABBITMQ_EXCHANGE || "talentflow.eventos",
  producer: "empleados-service"
});
const servicio = crearServicioEmpleados(repositorio, clienteDepartamentos, publicador);
const app = crearApp(servicio, clienteDepartamentos, {
  verificarBaseDeDatos: () => pool.query("SELECT 1"),
  publicador
});

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

publicador.iniciar();

app.listen(PUERTO, () => {
  console.log(`Servicio de empleados escuchando en http://localhost:${PUERTO}`);
  console.log(`Documentación Swagger en http://localhost:8080/empleados/docs (a través del Gateway)`);
});
