// Métricas Prometheus del Gateway (Reto 8, D5). Infraestructura (D7): no es lógica de
// negocio, por eso no entra en el umbral de cobertura; tiene su prueba (tests/metricas.test.js).
const express = require('express');
const client = require('prom-client');

// Hasta 10 s: el umbral de la alerta de latencia (2 s) cae dentro de un bucket exacto.
const BUCKETS_SEGUNDOS = [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10];

/**
 * Registro propio (no el global) con las métricas de proceso por defecto y el contrato
 * común de los 7 servicios: http_requests_total y http_request_duration_seconds.
 */
function crearMetricas() {
    const registro = new client.Registry();
    client.collectDefaultMetrics({ register: registro });
    const etiquetas = ['method', 'route', 'status'];
    const peticiones = new client.Counter({
        name: 'http_requests_total',
        help: 'Peticiones HTTP recibidas',
        labelNames: etiquetas,
        registers: [registro]
    });
    const duracion = new client.Histogram({
        name: 'http_request_duration_seconds',
        help: 'Duración de las peticiones HTTP en segundos',
        labelNames: etiquetas,
        buckets: BUCKETS_SEGUNDOS,
        registers: [registro]
    });
    return { registro, peticiones, duracion };
}

/**
 * `route` de baja cardinalidad: el Gateway no tiene rutas de Express (es un proxy), así
 * que se usa el prefijo del servicio (/empleados, /auth...), /health u "other". Nunca la
 * URL real: cada id distinto crearía una serie nueva.
 */
function rutaDelGateway(ruta, prefijos) {
    if (ruta === '/health') return '/health';
    return prefijos.find((p) => ruta === p || ruta.startsWith(`${p}/`)) ?? 'other';
}

// Middleware: mide cada petición al terminar la respuesta. La ruta se calcula al entrar,
// antes de que otro middleware toque req.url.
function middlewareMetricas({ peticiones, duracion }, prefijos) {
    return (req, res, next) => {
        const route = rutaDelGateway(req.path, prefijos);
        const fin = duracion.startTimer();
        res.on('finish', () => {
            const etiquetas = { method: req.method, route, status: String(res.statusCode) };
            peticiones.inc(etiquetas);
            fin(etiquetas);
        });
        next();
    };
}

/**
 * App aparte para /metrics, escuchada en un puerto interno (METRICS_PORT, solo `expose`
 * en Compose): el puerto público 8080 nunca publica las métricas (Reto 8, D13).
 */
function crearAppMetricas(registro) {
    const app = express();
    app.get('/metrics', async (req, res) => {
        try {
            res.set('Content-Type', registro.contentType).send(await registro.metrics());
        } catch (error) {
            console.error(`[Gateway Error] métricas: ${error.message}`);
            res.status(500).end();
        }
    });
    return app;
}

module.exports = { crearMetricas, rutaDelGateway, middlewareMetricas, crearAppMetricas, BUCKETS_SEGUNDOS };
