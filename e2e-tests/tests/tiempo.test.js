'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const { fechaHoraEnZona, planDelCiclo, periodoInmediato } = require('../support/tiempo');
const { leerConfiguracion } = require('../support/config');

test('formatea la hora local de Bogotá (UTC-5), no la de la máquina', () => {
  // 2026-10-17T20:30:05Z son las 15:30:05 en Bogotá.
  assert.equal(fechaHoraEnZona(Date.UTC(2026, 9, 17, 20, 30, 5), 'America/Bogota'), '2026-10-17T15:30:05');
  assert.equal(fechaHoraEnZona(Date.UTC(2026, 9, 17, 20, 30, 5), 'UTC'), '2026-10-17T20:30:05');
});

test('cambia de día según la zona y usa 00 para la medianoche', () => {
  // 03:00Z del 18 son las 22:00 del 17 en Bogotá; 05:00Z son las 00:00 del 18.
  assert.equal(fechaHoraEnZona(Date.UTC(2026, 9, 18, 3, 0, 0), 'America/Bogota'), '2026-10-17T22:00:00');
  assert.equal(fechaHoraEnZona(Date.UTC(2026, 9, 18, 5, 0, 0), 'America/Bogota'), '2026-10-18T00:00:00');
});

test('con el cron de la suite (5 s) el ciclo cabe en menos de 30 s de espera', () => {
  const plan = planDelCiclo({ periodoSchedulerS: 5, intervaloMs: 1000 });
  assert.equal(plan.adelantoS, 3);
  assert.equal(plan.duracionS, 10);
  assert.equal(plan.esperaInicioS, 18);
  assert.equal(plan.esperaFinS, 25);
  assert.equal(plan.intentosInicio, 19);
  assert.equal(plan.intentosFin, 26);
  assert.equal(plan.timeoutPasoMs, 56_000);
});

test('con el cron por defecto (60 s) el período dura más que una pasada y las esperas crecen', () => {
  const plan = planDelCiclo({ periodoSchedulerS: 60, intervaloMs: 1000 });
  assert.ok(plan.duracionS > 60, 'la pasada que inicia el período no puede finalizarlo');
  assert.equal(plan.esperaInicioS, 73);
  assert.equal(plan.esperaFinS, 135);
  assert.equal(plan.intentosFin, 136);
  assert.equal(plan.timeoutPasoMs, 166_000);
});

test('los intentos se redondean hacia arriba según el intervalo de polling', () => {
  const plan = planDelCiclo({ periodoSchedulerS: 5, intervaloMs: 2000 });
  assert.equal(plan.intentosInicio, 10); // ceil(18 / 2) + 1
  assert.equal(plan.intentosFin, 14); // ceil(25 / 2) + 1
});

test('rechaza un período o un intervalo que no son enteros positivos', () => {
  assert.throws(() => planDelCiclo({ periodoSchedulerS: 0, intervaloMs: 1000 }), /periodoSchedulerS/);
  assert.throws(() => planDelCiclo({ periodoSchedulerS: 5, intervaloMs: 0 }), /intervaloMs/);
});

test('el período inmediato empieza en un segundo exacto, con adelanto, y dura lo pedido', () => {
  const ahora = Date.UTC(2026, 9, 17, 20, 30, 5, 750);
  const periodo = periodoInmediato(ahora, 'America/Bogota', { adelantoS: 3, duracionS: 10 });
  assert.equal(periodo.fechaInicio, '2026-10-17T15:30:08');
  assert.equal(periodo.fechaFin, '2026-10-17T15:30:18');
  assert.equal(periodo.finMs - periodo.inicioMs, 10_000);
});

test('la configuración de vacaciones sale del entorno, con 5 s y Bogotá por defecto', () => {
  assert.deepEqual(leerConfiguracion({}).vacaciones, { periodoSchedulerS: 5, zonaHoraria: 'America/Bogota' });
  assert.deepEqual(leerConfiguracion({ E2E_SCHEDULER_SEGUNDOS: '60', E2E_ZONA_HORARIA: 'UTC' }).vacaciones, {
    periodoSchedulerS: 60,
    zonaHoraria: 'UTC',
  });
  assert.throws(() => leerConfiguracion({ E2E_SCHEDULER_SEGUNDOS: '-5' }), /E2E_SCHEDULER_SEGUNDOS/);
  assert.throws(() => leerConfiguracion({ E2E_ZONA_HORARIA: 'Marte/Olympus' }), /E2E_ZONA_HORARIA/);
});
