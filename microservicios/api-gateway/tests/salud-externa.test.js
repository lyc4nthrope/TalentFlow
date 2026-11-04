// Reto 8, D18-D20: salud de cada servicio vista desde fuera (por el Gateway) y como métrica.
// Servidores HTTP reales: lo que se garantiza es que el Gateway respete el código y el
// cuerpo que responde cada servicio, y eso vive en HTTP.
const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');

const { leerSaludDe, saludDeTodos, valoresDeSalud } = require('../salud');
const { crearApp } = require('../app');
const { crearMetricas, registrarMetricasSalud } = require('../metricas');

const SANO = { status: 'UP', service: 'a-service', checks: { database: 'UP', messageBroker: 'UP' } };
const BD_CAIDA = { status: 'DOWN', service: 'b-service', checks: { database: 'DOWN', messageBroker: 'UP' } };

// Responde /health con el código y el cuerpo dados (cuerpo string = no JSON).
function servicioDeSalud(codigo, cuerpo) {
    return http.createServer((req, res) => {
        res.writeHead(codigo, { 'content-type': 'application/json' });
        res.end(typeof cuerpo === 'string' ? cuerpo : JSON.stringify(cuerpo));
    });
}

const escuchar = (servidor) =>
    new Promise((resolve) => servidor.listen(0, '127.0.0.1', () => resolve(`http://127.0.0.1:${servidor.address().port}`)));

// Un puerto cerrado: nadie escucha ahí.
const CAIDO = 'http://127.0.0.1:1';

describe('salud de cada servicio desde fuera', () => {
    const servidores = [];
    let urlSano;
    let urlBdCaida;
    let urlNoJson;

    before(async () => {
        const sano = servicioDeSalud(200, SANO);
        const bdCaida = servicioDeSalud(503, BD_CAIDA);
        const noJson = servicioDeSalud(200, '<html>no es json</html>');
        servidores.push(sano, bdCaida, noJson);
        urlSano = await escuchar(sano);
        urlBdCaida = await escuchar(bdCaida);
        urlNoJson = await escuchar(noJson);
    });

    after(() => servidores.forEach((s) => s.close()));

    describe('leerSaludDe', () => {
        it('devuelve el código y el cuerpo del servicio tal cual (200)', async () => {
            const r = await leerSaludDe({ nombre: 'a-service', url: urlSano });
            assert.deepEqual(r, { codigo: 200, cuerpo: SANO });
        });

        it('respeta el 503 del servicio con su cuerpo (BD caída)', async () => {
            const r = await leerSaludDe({ nombre: 'b-service', url: urlBdCaida });
            assert.deepEqual(r, { codigo: 503, cuerpo: BD_CAIDA });
        });

        it('un servicio que no responde es 503 DOWN con su nombre, sin lanzar', async () => {
            const r = await leerSaludDe({ nombre: 'c-service', url: CAIDO });
            assert.deepEqual(r, { codigo: 503, cuerpo: { status: 'DOWN', service: 'c-service', detalle: 'No responde' } });
        });

        it('una respuesta que no es JSON cuenta como que no responde', async () => {
            const r = await leerSaludDe({ nombre: 'd-service', url: urlNoJson });
            assert.equal(r.codigo, 503);
            assert.equal(r.cuerpo.status, 'DOWN');
        });

        it('un JSON que no es un objeto (null, un número) cuenta como que no responde', async () => {
            for (const cuerpo of ['null', '42']) {
                const raro = servicioDeSalud(200, cuerpo);
                const url = await escuchar(raro);
                try {
                    const r = await leerSaludDe({ nombre: 'f-service', url });
                    assert.deepEqual(r, { codigo: 503, cuerpo: { status: 'DOWN', service: 'f-service', detalle: 'No responde' } });
                } finally {
                    raro.close();
                }
            }
        });

        it('corta a tiempo un servicio que no contesta', async () => {
            const colgado = http.createServer(() => {}); // acepta y nunca responde
            const url = await escuchar(colgado);
            try {
                const inicio = Date.now();
                const r = await leerSaludDe({ nombre: 'e-service', url }, { timeoutMs: 100 });
                assert.equal(r.codigo, 503);
                assert.ok(Date.now() - inicio < 2000);
            } finally {
                colgado.closeAllConnections();
                colgado.close();
            }
        });
    });

    describe('saludDeTodos', () => {
        it('200 y UP cuando todos están UP', async () => {
            const r = await saludDeTodos([{ nombre: 'a-service', url: urlSano }]);
            assert.equal(r.codigo, 200);
            assert.equal(r.cuerpo.status, 'UP');
            assert.equal(r.cuerpo.service, 'api-gateway');
            assert.deepEqual(r.cuerpo.servicios, { 'a-service': SANO });
            assert.ok(r.cuerpo.timestamp);
        });

        it('503 y DOWN si un servicio tiene la BD caída, con el detalle de cada uno', async () => {
            const r = await saludDeTodos([
                { nombre: 'a-service', url: urlSano },
                { nombre: 'b-service', url: urlBdCaida }
            ]);
            assert.equal(r.codigo, 503);
            assert.equal(r.cuerpo.status, 'DOWN');
            assert.deepEqual(r.cuerpo.servicios['b-service'], BD_CAIDA);
            assert.deepEqual(r.cuerpo.checks, { 'a-service': 'UP', 'b-service': 'DOWN' });
        });

        it('503 si un servicio no responde', async () => {
            const r = await saludDeTodos([{ nombre: 'c-service', url: CAIDO }]);
            assert.equal(r.codigo, 503);
            assert.deepEqual(r.cuerpo.checks, { 'c-service': 'DOWN' });
        });
    });

    describe('valoresDeSalud', () => {
        it('un 1/0 por servicio (check "status") y por cada dependencia de su checks', () => {
            const valores = valoresDeSalud({ 'a-service': SANO, 'b-service': BD_CAIDA });
            assert.deepEqual(valores, [
                { labels: { service: 'a-service', check: 'status' }, value: 1 },
                { labels: { service: 'a-service', check: 'database' }, value: 1 },
                { labels: { service: 'a-service', check: 'messageBroker' }, value: 1 },
                { labels: { service: 'b-service', check: 'status' }, value: 0 },
                { labels: { service: 'b-service', check: 'database' }, value: 0 },
                { labels: { service: 'b-service', check: 'messageBroker' }, value: 1 }
            ]);
        });

        it('un checks que no es un objeto se ignora (solo queda el status)', () => {
            const valores = valoresDeSalud({ 'g-service': { status: 'UP', checks: 'UP' } });
            assert.deepEqual(valores, [{ labels: { service: 'g-service', check: 'status' }, value: 1 }]);
        });

        it('un servicio que no responde solo tiene status 0 (sus dependencias no se conocen)', () => {
            const valores = valoresDeSalud({ 'c-service': { status: 'DOWN', detalle: 'No responde' } });
            assert.deepEqual(valores, [{ labels: { service: 'c-service', check: 'status' }, value: 0 }]);
        });
    });

    describe('rutas del Gateway', () => {
        let gateway;
        let base;

        before(async () => {
            const servicios = [
                { nombre: 'a-service', prefijo: '/a', url: urlSano },
                { nombre: 'b-service', prefijo: '/b', url: urlBdCaida }
            ];
            gateway = crearApp({ jwtSecret: 's', jwtIssuer: 'i', servicios }).listen(0, '127.0.0.1');
            await new Promise((r) => gateway.once('listening', r));
            base = `http://127.0.0.1:${gateway.address().port}`;
        });

        after(() => gateway.close());

        it('GET /health/{servicio} sin token devuelve el /health del servicio con su 200', async () => {
            const r = await fetch(`${base}/health/a-service`);
            assert.equal(r.status, 200);
            assert.deepEqual(await r.json(), SANO);
        });

        it('GET /health/{servicio} devuelve el 503 del servicio con la BD caída', async () => {
            const r = await fetch(`${base}/health/b-service`);
            assert.equal(r.status, 503);
            assert.deepEqual(await r.json(), BD_CAIDA);
        });

        it('GET /health/{servicio} desconocido es 404 y lista los servicios válidos', async () => {
            const r = await fetch(`${base}/health/no-existe`);
            assert.equal(r.status, 404);
            assert.deepEqual((await r.json()).servicios, ['a-service', 'b-service']);
        });

        it('nombres especiales o codificados no son servicios: 404', async () => {
            for (const nombre of ['constructor', '__proto__', 'toString', 'a-service%2F..']) {
                const r = await fetch(`${base}/health/${nombre}`);
                assert.equal(r.status, 404, nombre);
            }
        });

        it('GET /health/servicios es 200 y UP cuando todos están UP', async () => {
            const soloSano = crearApp({
                jwtSecret: 's',
                jwtIssuer: 'i',
                servicios: [{ nombre: 'a-service', prefijo: '/a', url: urlSano }]
            }).listen(0, '127.0.0.1');
            await new Promise((r) => soloSano.once('listening', r));
            try {
                const r = await fetch(`http://127.0.0.1:${soloSano.address().port}/health/servicios`);
                assert.equal(r.status, 200);
                assert.equal((await r.json()).status, 'UP');
            } finally {
                soloSano.close();
            }
        });

        it('GET /health/servicios es 503 si alguno está DOWN', async () => {
            const r = await fetch(`${base}/health/servicios`);
            assert.equal(r.status, 503);
            const cuerpo = await r.json();
            assert.equal(cuerpo.status, 'DOWN');
            assert.deepEqual(Object.keys(cuerpo.servicios), ['a-service', 'b-service']);
        });

        it('GET /health sigue en 200 aunque un servicio esté DOWN (healthcheck de Docker)', async () => {
            const r = await fetch(`${base}/health`);
            assert.equal(r.status, 200);
        });
    });

    describe('métrica talentflow_health_check', () => {
        it('se calcula en cada scrape con el /health real de cada servicio', async () => {
            const { registro } = crearMetricas();
            registrarMetricasSalud(registro, [
                { nombre: 'a-service', url: urlSano },
                { nombre: 'b-service', url: urlBdCaida }
            ]);
            const texto = await registro.metrics();
            assert.match(texto, /talentflow_health_check\{service="a-service",check="database"\} 1/);
            assert.match(texto, /talentflow_health_check\{service="b-service",check="database"\} 0/);
            assert.match(texto, /talentflow_health_check\{service="b-service",check="status"\} 0/);
        });

        it('un servicio que responde null no tumba el scrape del Gateway', async () => {
            const raro = servicioDeSalud(200, 'null');
            const url = await escuchar(raro);
            try {
                const { registro } = crearMetricas();
                registrarMetricasSalud(registro, [{ nombre: 'f-service', url }]);
                const texto = await registro.metrics();
                assert.match(texto, /talentflow_health_check\{service="f-service",check="status"\} 0/);
                assert.match(texto, /process_cpu_seconds_total/); // el resto de métricas sigue saliendo
            } finally {
                raro.close();
            }
        });

        it('no deja valores viejos: un servicio que deja de responder pierde sus dependencias', async () => {
            const { registro } = crearMetricas();
            const servicios = [{ nombre: 'a-service', url: urlSano }];
            registrarMetricasSalud(registro, servicios);
            await registro.metrics();
            servicios[0].url = CAIDO;
            const texto = await registro.metrics();
            assert.match(texto, /talentflow_health_check\{service="a-service",check="status"\} 0/);
            assert.doesNotMatch(texto, /check="database"/);
        });
    });
});
