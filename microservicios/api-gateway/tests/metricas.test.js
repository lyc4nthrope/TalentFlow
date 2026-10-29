// Reto 8, D5 y D13: contrato de métricas HTTP del Gateway y su exposición en un puerto aparte.
const { describe, it } = require('node:test');
const assert = require('node:assert/strict');

const { crearApp } = require('../app');
const { crearMetricas, rutaDelGateway, crearAppMetricas } = require('../metricas');

const PREFIJOS = ['/auth', '/empleados'];

async function conServidor(app, fn) {
    const server = await new Promise((resolve) => {
        const s = app.listen(0, '127.0.0.1', () => resolve(s));
    });
    try {
        return await fn(`http://127.0.0.1:${server.address().port}`);
    } finally {
        server.close();
    }
}

describe('rutaDelGateway', () => {
    it('usa el prefijo del servicio, nunca la URL real', () => {
        assert.equal(rutaDelGateway('/empleados', PREFIJOS), '/empleados');
        assert.equal(rutaDelGateway('/empleados/E001', PREFIJOS), '/empleados');
        assert.equal(rutaDelGateway('/auth/login', PREFIJOS), '/auth');
    });

    it('/health se mide aparte y lo demás cae en "other"', () => {
        assert.equal(rutaDelGateway('/health', PREFIJOS), '/health');
        assert.equal(rutaDelGateway('/empleadosX', PREFIJOS), 'other');
        assert.equal(rutaDelGateway('/metrics', PREFIJOS), 'other');
    });
});

describe('métricas del Gateway', () => {
    const servicios = [{ nombre: 'empleados-service', prefijo: '/empleados', url: 'http://127.0.0.1:1' }];

    it('cuenta y mide cada petición con method, route y status; /metrics no está en el puerto público', async () => {
        const metricas = crearMetricas();
        const app = crearApp({
            jwtSecret: 's',
            jwtIssuer: 'auth-service',
            servicios,
            salud: async () => ({ status: 'UP' }),
            metricas
        });

        await conServidor(app, async (base) => {
            assert.equal((await fetch(`${base}/health`)).status, 200);
            assert.equal((await fetch(`${base}/empleados/E001`)).status, 401);
            assert.equal((await fetch(`${base}/metrics`)).status, 404);
        });

        const texto = await metricas.registro.metrics();
        assert.match(texto, /http_requests_total\{method="GET",route="\/health",status="200"\} 1/);
        assert.match(texto, /http_requests_total\{method="GET",route="\/empleados",status="401"\} 1/);
        assert.match(texto, /http_requests_total\{method="GET",route="other",status="404"\} 1/);
        assert.match(texto, /http_request_duration_seconds_bucket\{le="2",method="GET",route="\/health",status="200"\} 1/);
        assert.match(texto, /process_cpu_seconds_total/);
        assert.match(texto, /nodejs_eventloop_lag_seconds/);
    });

    it('la app de métricas publica el formato de Prometheus en /metrics', async () => {
        const metricas = crearMetricas();
        await conServidor(crearAppMetricas(metricas.registro), async (base) => {
            const respuesta = await fetch(`${base}/metrics`);
            assert.equal(respuesta.status, 200);
            assert.match(respuesta.headers.get('content-type'), /^text\/plain/);
            assert.match(await respuesta.text(), /# TYPE http_requests_total counter/);
        });
    });

    it('un fallo al serializar responde 500', async () => {
        const registro = { contentType: 'text/plain', metrics: async () => { throw new Error('x'); } };
        const original = console.error;
        console.error = () => {};
        try {
            await conServidor(crearAppMetricas(registro), async (base) => {
                assert.equal((await fetch(`${base}/metrics`)).status, 500);
            });
        } finally {
            console.error = original;
        }
    });
});
