'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const { diagnosticoRetiro, ventanaDeAusencia, problemasDeFechaRetiro } = require('../support/retiro');

test('el retiro se cumple solo con el empleado RETIRADO y el login en 401', () => {
  assert.deepEqual(diagnosticoRetiro({ estadoEmpleado: 'RETIRADO', login: 401 }), { cumplida: true });
});

test('si el empleado sigue ACTIVO, el diagnóstico apunta a gestion-empleados', () => {
  const { cumplida, motivo } = diagnosticoRetiro({ estadoEmpleado: 'ACTIVO', login: 200 });
  assert.equal(cumplida, false);
  assert.match(motivo, /no se aplicó en gestion-empleados: el empleado sigue ACTIVO/);
});

test('un 401 sin el empleado retirado no cuenta: no prueba la causa', () => {
  const { cumplida, motivo } = diagnosticoRetiro({ estadoEmpleado: 'ACTIVO', login: 401 });
  assert.equal(cumplida, false);
  assert.match(motivo, /ya falla sin que el empleado esté retirado/);
});

test('si empleados lo retiró y el login sigue en 200, el diagnóstico apunta a auth', () => {
  const { cumplida, motivo } = diagnosticoRetiro({ estadoEmpleado: 'RETIRADO', login: 200 });
  assert.equal(cumplida, false);
  assert.match(motivo, /la cuenta sigue activa/);
  assert.match(motivo, /auth-service: ¿consumió empleado.retirado/);
});

test('un código inesperado del login con el empleado retirado se informa tal cual', () => {
  const { motivo } = diagnosticoRetiro({ estadoEmpleado: 'RETIRADO', login: 500 });
  assert.match(motivo, /responde 500 y se esperaba 401/);
});

test('la ventana de ausencia nunca baja de la mínima', () => {
  assert.equal(ventanaDeAusencia({ minimaMs: 3000 }), 3000);
  assert.equal(ventanaDeAusencia({ minimaMs: 3000, latenciaControlMs: 20 }), 3000);
});

test('si el control positivo tardó, la ventana es el triple de lo que tardó, con tope', () => {
  assert.equal(ventanaDeAusencia({ minimaMs: 3000, latenciaControlMs: 1500 }), 4500);
  assert.equal(ventanaDeAusencia({ minimaMs: 3000, maximaMs: 10_000, latenciaControlMs: 9000 }), 10_000);
  // El tope nunca deja la ventana por debajo de la mínima.
  assert.equal(ventanaDeAusencia({ minimaMs: 3000, maximaMs: 1000, latenciaControlMs: 9000 }), 3000);
});

test('la ventana de ausencia rechaza datos sin sentido', () => {
  assert.throws(() => ventanaDeAusencia({ minimaMs: 0 }), /minimaMs/);
  assert.throws(() => ventanaDeAusencia({ minimaMs: 3000, latenciaControlMs: -1 }), /latenciaControlMs/);
});

// 20:30:05Z del 17 son las 15:30:05 del 17 en Bogotá.
const RETIRO = { desdeMs: Date.UTC(2026, 9, 17, 20, 30, 4, 900), hastaMs: Date.UTC(2026, 9, 17, 20, 30, 5, 100), zona: 'America/Bogota' };

test('una fechaRetiro del momento del DELETE no tiene problemas, aunque venga sin milisegundos', () => {
  assert.deepEqual(problemasDeFechaRetiro('2026-10-17T20:30:04Z', RETIRO), []);
  assert.deepEqual(problemasDeFechaRetiro('2026-10-17T20:30:05.000Z', RETIRO), []);
});

test('una fechaRetiro de otro momento se informa con el intervalo esperado', () => {
  const [problema] = problemasDeFechaRetiro('2026-10-17T19:00:00Z', RETIRO);
  assert.match(problema, /no cae en el momento del retiro \(2026-10-17T20:30:04.900Z/);
});

test('el día se compara en la zona del negocio, no en UTC', () => {
  // 03:00:05Z del 18 son las 22:00:05 del 17 en Bogotá: es "hoy" (17) aunque en UTC sea 18.
  const noche = { desdeMs: Date.UTC(2026, 9, 18, 3, 0, 4), hastaMs: Date.UTC(2026, 9, 18, 3, 0, 6), zona: 'America/Bogota' };
  assert.deepEqual(problemasDeFechaRetiro('2026-10-18T03:00:05Z', noche), []);
  const problemas = problemasDeFechaRetiro('2026-10-17T03:00:05Z', noche);
  assert.ok(problemas.some((p) => /es del 2026-10-16 en America\/Bogota y se esperaba hoy \(2026-10-17\)/.test(p)), problemas.join(' | '));
});

test('una fechaRetiro que no es un instante UTC se rechaza sin interpretarla', () => {
  assert.match(problemasDeFechaRetiro(null, RETIRO)[0], /no es un instante UTC/);
  assert.match(problemasDeFechaRetiro('2026-10-17', RETIRO)[0], /no es un instante UTC/);
});
