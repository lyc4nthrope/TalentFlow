// Métricas Prometheus del servicio (Reto 8, D5). Infraestructura (D7): no es lógica de
// negocio, por eso no entra en el umbral de cobertura; tiene su prueba (tests/metricas.test.js).
const client = require("@prometheus-io/client");

// Hasta 10 s: el umbral de la alerta de latencia (2 s) cae dentro de un bucket exacto.
const BUCKETS_SEGUNDOS = [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10];

// Registro propio (no el global) con las métricas de proceso por defecto y el contrato
// común de los 7 servicios: http_requests_total y http_request_duration_seconds.
function crearMetricas() {
  const registro = new client.Registry();
  client.collectDefaultMetrics({ register: registro });
  const etiquetas = ["method", "route", "status"];
  const peticiones = new client.Counter({
    name: "http_requests_total",
    help: "Peticiones HTTP recibidas",
    labelNames: etiquetas,
    registers: [registro]
  });
  const duracion = new client.Histogram({
    name: "http_request_duration_seconds",
    help: "Duración de las peticiones HTTP en segundos",
    labelNames: etiquetas,
    buckets: BUCKETS_SEGUNDOS,
    registers: [registro]
  });
  return { registro, peticiones, duracion };
}

// `route` de baja cardinalidad: el patrón de Express (/empleados/:id), nunca la URL real
// (cada id crearía una serie nueva). Lo que no casa con ninguna ruta (404, Swagger) es "other".
function rutaDeExpress(req) {
  return req.route ? `${req.baseUrl}${req.route.path}` : "other";
}

// Middleware: mide cada petición cuando termina la respuesta (ya se sabe la ruta y el status).
function middlewareMetricas({ peticiones, duracion }) {
  return (req, res, next) => {
    const fin = duracion.startTimer();
    res.on("finish", () => {
      const etiquetas = { method: req.method, route: rutaDeExpress(req), status: String(res.statusCode) };
      peticiones.inc(etiquetas);
      fin(etiquetas);
    });
    next();
  };
}

// GET /metrics en el puerto interno del servicio (8081, solo `expose`): el Gateway solo
// enruta /empleados/*, así que desde fuera no es alcanzable (Reto 8, D13).
function manejadorMetricas(registro) {
  return async (req, res) => {
    try {
      res.set("Content-Type", registro.contentType).send(await registro.metrics());
    } catch (error) {
      console.error(`Error generando métricas: ${error.message}`);
      res.status(500).end();
    }
  };
}

module.exports = { crearMetricas, rutaDeExpress, middlewareMetricas, manejadorMetricas, BUCKETS_SEGUNDOS };
