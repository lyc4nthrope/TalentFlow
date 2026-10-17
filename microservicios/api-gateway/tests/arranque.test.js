// Reto 5, D11/D21: sin JWT_SECRET el Gateway no arranca (no hay secreto por defecto en el
// código). Se ejecuta index.js en un proceso hijo para ver el código de salida real.
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { spawn, spawnSync } = require('node:child_process');
const path = require('node:path');

const INDEX = path.join(__dirname, '..', 'index.js');

function arrancar(env) {
    return spawnSync(process.execPath, [INDEX], {
        env: { PATH: process.env.PATH, PORT: '0', ...env },
        encoding: 'utf8',
        timeout: 5000
    });
}

test('sin JWT_SECRET el Gateway termina con código 1 y no abre el puerto', () => {
    const r = arrancar({});
    assert.equal(r.status, 1, `salida ${r.status}, señal ${r.signal}`);
    assert.match(r.stderr, /JWT_SECRET/);
    assert.doesNotMatch(r.stdout, /ejecutandose en el puerto/);
});

test('con JWT_SECRET vacío tampoco arranca', () => {
    const r = arrancar({ JWT_SECRET: '' });
    assert.equal(r.status, 1);
});

test('con JWT_SECRET de solo espacios tampoco arranca', () => {
    for (const valor of ['   ', '\t\n']) {
        const r = arrancar({ JWT_SECRET: valor });
        assert.equal(r.status, 1, `JWT_SECRET=${JSON.stringify(valor)}: salida ${r.status}`);
        assert.match(r.stderr, /JWT_SECRET/);
    }
});

test('con JWT_SECRET el Gateway arranca y abre el puerto', async () => {
    const hijo = spawn(process.execPath, [INDEX], {
        env: { PATH: process.env.PATH, PORT: '0', JWT_SECRET: 'secreto-de-prueba' },
        stdio: ['ignore', 'pipe', 'pipe']
    });
    try {
        const escuchando = await new Promise((resolve, reject) => {
            const plazo = setTimeout(() => reject(new Error('no abrió el puerto en 5 s')), 5000);
            let salida = '';
            hijo.stdout.on('data', (trozo) => {
                salida += trozo;
                if (/ejecutandose en el puerto/.test(salida)) {
                    clearTimeout(plazo);
                    resolve(true);
                }
            });
            hijo.on('exit', (codigo) => {
                clearTimeout(plazo);
                reject(new Error(`terminó con código ${codigo} sin abrir el puerto`));
            });
        });
        assert.equal(escuchando, true);
        assert.equal(hijo.exitCode, null, 'el proceso debe seguir vivo');
    } finally {
        hijo.kill();
    }
});
