const { describe, it } = require('node:test');
const assert = require('node:assert/strict');

const { consultarSalud, detectarProblemas, saludDelSistema } = require('../salud');

// fetch falso: responde según la URL, o falla para simular un servicio caído.
function fetchFalso(respuestas) {
    return async (url) => {
        const cuerpo = respuestas[url.replace('/health', '')];
        if (cuerpo === undefined) throw new Error('ECONNREFUSED');
        return { json: async () => cuerpo };
    };
}

const SANO = { status: 'UP', components: { app: 'UP', db: 'UP', broker: 'UP' } };

describe('consultarSalud', () => {
    it('aplana los componentes del /health del servicio', async () => {
        const estado = await consultarSalud('http://s', { fetchImpl: fetchFalso({ 'http://s': SANO }) });
        assert.deepEqual(estado, { status: 'UP', app: 'UP', db: 'UP', broker: 'UP' });
    });

    it('un servicio que no responde queda DOWN, sin lanzar', async () => {
        const estado = await consultarSalud('http://caido', { fetchImpl: fetchFalso({}) });
        assert.deepEqual(estado, { status: 'DOWN', detalle: 'No responde' });
    });
});

describe('detectarProblemas', () => {
    it('sin problemas cuando todo está UP y el circuito CLOSED', () => {
        const problemas = detectarProblemas({
            'empleados-service': { status: 'UP', db: 'UP', broker: 'UP', circuitoDepartamentos: 'CLOSED' },
            'departamentos-service': { status: 'UP', db: 'UP' }
        });
        assert.deepEqual(problemas, []);
    });

    it('explica cada causa de degradación', () => {
        const problemas = detectarProblemas({
            'empleados-service': { status: 'UP', broker: 'UP', circuitoDepartamentos: 'OPEN' },
            'departamentos-service': { status: 'DOWN', detalle: 'No responde' },
            'perfiles-service': { status: 'DOWN', db: 'DOWN', broker: 'UP' },
            'vacaciones-service': { status: 'UP', db: 'UP', broker: 'DOWN' }
        });
        assert.deepEqual(problemas, [
            'empleados-service: Circuit Breaker hacia departamentos OPEN',
            'departamentos-service: No responde',
            'perfiles-service: su base de datos no responde',
            'vacaciones-service: sin conexión al broker de mensajería'
        ]);
    });
});

describe('saludDelSistema', () => {
    const servicios = [
        { nombre: 'a-service', url: 'http://a' },
        { nombre: 'b-service', url: 'http://b' }
    ];

    it('OK cuando todos los servicios están sanos', async () => {
        const salud = await saludDelSistema(servicios, { fetchImpl: fetchFalso({ 'http://a': SANO, 'http://b': SANO }) });
        assert.equal(salud.status, 'UP');
        assert.equal(salud.sistema, 'OK');
        assert.deepEqual(salud.problemas, []);
        assert.deepEqual(Object.keys(salud.servicios), ['a-service', 'b-service']);
    });

    it('DEGRADADO (pero el Gateway sigue UP) si un servicio cae', async () => {
        const salud = await saludDelSistema(servicios, { fetchImpl: fetchFalso({ 'http://a': SANO }) });
        assert.equal(salud.status, 'UP');
        assert.equal(salud.sistema, 'DEGRADADO');
        assert.deepEqual(salud.problemas, ['b-service: No responde']);
    });
});
