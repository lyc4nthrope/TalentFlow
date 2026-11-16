'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const { leerConfiguracion } = require('../support/config');

const cucumberPath = require.resolve('@cucumber/cucumber');
delete require.cache[cucumberPath];
require.cache[cucumberPath] = {
  id: cucumberPath,
  filename: cucumberPath,
  loaded: true,
  exports: {
    setWorldConstructor: () => {},
    setDefaultTimeout: () => {},
    World: class {},
    Given: () => {},
    When: () => {},
    Then: () => {},
    defineParameterType: () => {},
  },
};
const { programarPeriodo } = require('../support/vacaciones');

test('sin variables usa los valores académicos del Compose', () => {
  const config = leerConfiguracion({});
  assert.equal(config.baseUrl, 'http://localhost:8080');
  assert.equal(config.mailhogUrl, 'http://localhost:8025');
  assert.deepEqual(config.admin, { email: 'admin@empresa.com', password: 'Admin123*' });
  assert.deepEqual(config.polling, { intentos: 15, intervaloMs: 1000 });
});

test('toma BASE_URL, credenciales y polling del entorno', () => {
  const config = leerConfiguracion({
    BASE_URL: 'http://api-gateway:8080/',
    MAILHOG_URL: 'http://mailhog:8025/',
    ADMIN_EMAIL: 'otro@empresa.com',
    ADMIN_PASSWORD: 'Otra123*',
    E2E_POLL_INTENTOS: '5',
    E2E_POLL_INTERVALO_MS: '500',
  });
  assert.equal(config.baseUrl, 'http://api-gateway:8080');
  assert.equal(config.mailhogUrl, 'http://mailhog:8025');
  assert.deepEqual(config.admin, { email: 'otro@empresa.com', password: 'Otra123*' });
  assert.deepEqual(config.polling, { intentos: 5, intervaloMs: 500 });
});

test('falla con un mensaje claro si BASE_URL está mal escrita', () => {
  assert.throws(() => leerConfiguracion({ BASE_URL: 'localhost:8080' }), /BASE_URL debe usar http o https/);
  assert.throws(() => leerConfiguracion({ BASE_URL: 'no es url' }), /BASE_URL no es una URL válida: "no es url"/);
  assert.throws(() => leerConfiguracion({ MAILHOG_URL: 'ftp://mailhog' }), /MAILHOG_URL debe usar http o https/);
});

test('rechaza parámetros de polling que no son enteros positivos', () => {
  assert.throws(() => leerConfiguracion({ E2E_POLL_INTENTOS: '0' }), /E2E_POLL_INTENTOS/);
  assert.throws(() => leerConfiguracion({ E2E_POLL_INTERVALO_MS: '1.5' }), /E2E_POLL_INTERVALO_MS/);
});

test('la ventana mínima de las pruebas de ausencia es 3 s y se puede cambiar por entorno', () => {
  assert.deepEqual(leerConfiguracion({}).ausencia, { ventanaMinimaMs: 3000 });
  assert.deepEqual(leerConfiguracion({ E2E_VENTANA_AUSENCIA_MS: '5000' }).ausencia, { ventanaMinimaMs: 5000 });
  assert.throws(() => leerConfiguracion({ E2E_VENTANA_AUSENCIA_MS: '-1' }), /E2E_VENTANA_AUSENCIA_MS/);
});

test('programarPeriodo usa el token del administrador aunque la sesión actual sea de usuario', async () => {
  const mundo = {
    recurso: () => ({ id: 'E-42' }),
    tokenDeAdmin: async () => 'admin-token',
    peticion: async (_metodo, _ruta, opciones) => {
      assert.equal(opciones.token, 'admin-token');
      return { status: 201, body: { id: 'V-1' } };
    },
    enviar: async () => {
      throw new Error('no debería usarse enviar() cuando se programa el período');
    },
    recursos: new Map(),
  };

  const respuesta = await programarPeriodo(mundo, {
    fechaInicio: '2026-01-01T08:00:00',
    fechaFin: '2026-01-05T18:00:00',
  });

  assert.equal(respuesta.status, 201);
  assert.equal(mundo.recursos.get('vacaciones').id, 'V-1');
});
