'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const { diagnosticoSuspension, diagnosticoReactivacion } = require('../support/ciclo');

const vistos = (...estados) => new Set(estados);

test('la suspensión se cumple solo con el período EN_CURSO y el login en 401', () => {
  assert.deepEqual(diagnosticoSuspension({ estado: 'EN_CURSO', login: 401, vistos: vistos('EN_CURSO') }), {
    cumplida: true,
  });
});

test('si el período sigue PROGRAMADA, el diagnóstico apunta al scheduler', () => {
  const { cumplida, motivo } = diagnosticoSuspension({ estado: 'PROGRAMADA', login: 200, vistos: vistos('PROGRAMADA') });
  assert.equal(cumplida, false);
  assert.match(motivo, /nunca empezó/);
  assert.match(motivo, /scheduler/);
});

test('un 401 con el período todavía PROGRAMADA no cuenta como suspensión', () => {
  const { cumplida, motivo } = diagnosticoSuspension({ estado: 'PROGRAMADA', login: 401, vistos: vistos('PROGRAMADA') });
  assert.equal(cumplida, false);
  assert.match(motivo, /ya falla sin que el período haya empezado/);
});

test('si el período empezó y el login sigue en 200, el diagnóstico apunta a auth', () => {
  const { cumplida, motivo } = diagnosticoSuspension({ estado: 'EN_CURSO', login: 200, vistos: vistos('PROGRAMADA', 'EN_CURSO') });
  assert.equal(cumplida, false);
  assert.match(motivo, /empezó \(EN_CURSO\) pero la cuenta sigue activa/);
  assert.match(motivo, /auth-service/);
});

test('un código inesperado del login con el período EN_CURSO se informa tal cual', () => {
  const { motivo } = diagnosticoSuspension({ estado: 'EN_CURSO', login: 500, vistos: vistos('EN_CURSO') });
  assert.match(motivo, /responde 500 y se esperaba 401/);
});

test('si el período empezó y terminó sin un 401, la cuenta siguió activa: apunta a auth', () => {
  const { motivo } = diagnosticoSuspension({
    estado: 'FINALIZADA',
    login: 200,
    vistos: vistos('PROGRAMADA', 'EN_CURSO', 'FINALIZADA'),
  });
  assert.match(motivo, /empezó \(EN_CURSO\) y quedó FINALIZADA sin que el login respondiera 401/);
  assert.match(motivo, /vacaciones\.iniciadas/);
});

test('si el período terminó sin verlo EN_CURSO, lo dice con los estados vistos', () => {
  const { motivo } = diagnosticoSuspension({ estado: 'FINALIZADA', login: 200, vistos: vistos('PROGRAMADA', 'FINALIZADA') });
  assert.match(motivo, /quedó FINALIZADA sin que se observara/);
  assert.match(motivo, /PROGRAMADA → FINALIZADA/);
});

test('la reactivación se cumple con el período FINALIZADA y el login en 200', () => {
  assert.deepEqual(diagnosticoReactivacion({ estado: 'FINALIZADA', login: 200 }), { cumplida: true });
});

test('si el período finalizó y el login sigue fallando, el diagnóstico apunta a auth', () => {
  const { cumplida, motivo } = diagnosticoReactivacion({ estado: 'FINALIZADA', login: 401 });
  assert.equal(cumplida, false);
  assert.match(motivo, /finalizó \(FINALIZADA\) pero el login sigue respondiendo 401/);
  assert.match(motivo, /vacaciones\.finalizadas/);
});

test('si el período sigue EN_CURSO, el diagnóstico de la reactivación apunta al scheduler', () => {
  const { motivo } = diagnosticoReactivacion({ estado: 'EN_CURSO', login: 401 });
  assert.match(motivo, /nunca finalizó: sigue EN_CURSO/);
  assert.match(motivo, /scheduler/);
});

test('un estado inesperado en la reactivación se informa tal cual', () => {
  const { motivo } = diagnosticoReactivacion({ estado: 'CANCELADA', login: 200 });
  assert.match(motivo, /quedó CANCELADA y se esperaba FINALIZADA/);
});
