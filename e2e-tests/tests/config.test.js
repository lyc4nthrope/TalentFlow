'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const { leerConfiguracion } = require('../support/config');

test('sin variables usa los valores académicos del Compose', () => {
  const config = leerConfiguracion({});
  assert.equal(config.baseUrl, 'http://localhost:8080');
  assert.equal(config.mailhogUrl, 'http://localhost:8025');
  assert.deepEqual(config.admin, { email: 'admin@empresa.com', password: 'Admin123*' });
  assert.deepEqual(config.polling, { intentos: 15, intervaloMs: 2000 });
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
