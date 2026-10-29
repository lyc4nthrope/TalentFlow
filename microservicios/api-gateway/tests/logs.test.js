const { test } = require('node:test');
const assert = require('node:assert/strict');
const { lineaJson, activarLogsJson } = require('../logs');

const spanFalso = {
    spanContext: () => ({ traceId: '4bf92f3577b34da6a3ce929d0e0e4736', spanId: '00f067aa0ba902b7', traceFlags: 1 })
};

test('línea JSON con el formato común y el traceId del span activo', () => {
    const linea = JSON.parse(lineaJson('INFO', ['creado %s', 'E001'], 'mi-servicio', spanFalso));
    assert.equal(linea.level, 'INFO');
    assert.equal(linea.service, 'mi-servicio');
    assert.equal(linea.message, 'creado E001');
    assert.equal(linea.traceId, '4bf92f3577b34da6a3ce929d0e0e4736');
    assert.equal(linea.spanId, '00f067aa0ba902b7');
    assert.ok(!Number.isNaN(Date.parse(linea.timestamp)));
});

test('sin span activo no inventa traceId', () => {
    const linea = JSON.parse(lineaJson('WARN', ['aviso'], 'x', undefined));
    assert.equal(linea.traceId, undefined);
});

test('console.* queda en JSON: info a stdout, error a stderr', () => {
    const escrito = { stdout: [], stderr: [] };
    let alAvisar;
    const proceso = {
        stdout: { write: (t) => escrito.stdout.push(t) },
        stderr: { write: (t) => escrito.stderr.push(t) },
        on: (evento, fn) => { if (evento === 'warning') alAvisar = fn; }
    };
    const consola = {};
    activarLogsJson('x', consola, proceso);
    alAvisar({ name: 'DeprecationWarning', message: 'util._extend' });
    assert.equal(JSON.parse(escrito.stderr.shift()).level, 'WARN');
    consola.log('hola');
    consola.error(new Error('falló'));
    assert.equal(JSON.parse(escrito.stdout[0]).message, 'hola');
    const error = JSON.parse(escrito.stderr[0]);
    assert.equal(error.level, 'ERROR');
    assert.match(error.message, /falló/);
});
