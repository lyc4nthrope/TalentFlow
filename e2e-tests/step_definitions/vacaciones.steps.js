'use strict';

const { Given, When, Then, defineParameterType } = require('@cucumber/cucumber');
const { configuracion } = require('../support/config');
const { exigirCodigo, extracto, ALIAS_USUARIO } = require('../support/world');
const { eventualmente } = require('../support/polling');
const { planDelCiclo, periodoInmediato } = require('../support/tiempo');
const { diagnosticoSuspension, diagnosticoReactivacion } = require('../support/ciclo');
const {
  fechaHoraEnDias,
  programarPeriodo,
  consultarPeriodo,
  periodosDelEmpleado,
  codigoDeLogin,
  esperarNotificacion,
} = require('../support/vacaciones');

// Vacaciones (P5, F9). El empleado es el recurso "empleado" de los Antecedentes y el
// período programado queda en "vacaciones".

// Fechas y esperas del ciclo, derivadas del período del scheduler (D6). Las esperas
// del scheduler necesitan su propio límite de Cucumber: con el cron por defecto (60 s)
// superan el de world.js.
const PLAN = planDelCiclo({
  periodoSchedulerS: configuracion.vacaciones.periodoSchedulerS,
  intervaloMs: configuracion.polling.intervaloMs,
});
const ESPERA_DEL_SCHEDULER = { timeout: PLAN.timeoutPasoMs };

// Períodos lejanos (el scheduler no los toca durante la prueba), con hora.
const DIAS_HASTA_EL_PERIODO = 60;

// Fechas incoherentes, nombradas por lo que son (Esquema del escenario).
const FECHAS_INCOHERENTES = {
  'la fecha de fin es anterior a la de inicio': (mundo) => ({
    fechaInicio: fechaHoraEnDias(mundo, DIAS_HASTA_EL_PERIODO + 4),
    fechaFin: fechaHoraEnDias(mundo, DIAS_HASTA_EL_PERIODO, '18:00:00'),
  }),
  'la fecha de fin es igual a la de inicio': (mundo) => ({
    fechaInicio: fechaHoraEnDias(mundo, DIAS_HASTA_EL_PERIODO),
    fechaFin: fechaHoraEnDias(mundo, DIAS_HASTA_EL_PERIODO),
  }),
  'la fecha de inicio ya pasó': (mundo) => ({
    fechaInicio: fechaHoraEnDias(mundo, -1),
    fechaFin: fechaHoraEnDias(mundo, DIAS_HASTA_EL_PERIODO, '18:00:00'),
  }),
};

const escaparRegExp = (texto) => texto.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

defineParameterType({
  name: 'fechas incoherentes',
  regexp: new RegExp(Object.keys(FECHAS_INCOHERENTES).map(escaparRegExp).join('|')),
  transformer: (frase) => frase,
});

function periodoDelEscenario(mundo) {
  return mundo.recurso('vacaciones');
}

function empleadoDelEscenario(mundo) {
  return mundo.recurso(ALIAS_USUARIO);
}

// --- Programación y validaciones ----------------------------------------------

Given('que el empleado ya tiene un período de vacaciones programado', async function () {
  const respuesta = await programarPeriodo(
    this,
    {
      fechaInicio: fechaHoraEnDias(this, DIAS_HASTA_EL_PERIODO),
      fechaFin: fechaHoraEnDias(this, DIAS_HASTA_EL_PERIODO + 4, '18:00:00'),
    },
    { alias: 'vacaciones existentes', registrar: false }
  );
  exigirCodigo(respuesta, 201, 'Programar el período que después se cruza');
});

When('programo un período de vacaciones para el empleado dentro de {int} días', async function (dias) {
  await programarPeriodo(this, {
    fechaInicio: fechaHoraEnDias(this, dias),
    fechaFin: fechaHoraEnDias(this, dias + 4, '18:00:00'),
  });
});

When('programo un período de vacaciones en el que {fechas incoherentes}', async function (caso) {
  await programarPeriodo(this, FECHAS_INCOHERENTES[caso](this));
});

// Empieza dos días después del existente y termina dos días después de él.
When('programo un período de vacaciones que se cruza con el ya programado', async function () {
  await programarPeriodo(this, {
    fechaInicio: fechaHoraEnDias(this, DIAS_HASTA_EL_PERIODO + 2),
    fechaFin: fechaHoraEnDias(this, DIAS_HASTA_EL_PERIODO + 6, '18:00:00'),
  });
});

// Se relee el período: el 201 no basta, tiene que haber quedado guardado así.
Then('el período debe quedar en estado {string}', async function (esperado) {
  const { id } = periodoDelEscenario(this);
  const periodo = await consultarPeriodo(this, id);
  if (periodo.estado !== esperado || periodo.empleadoId !== empleadoDelEscenario(this).id) {
    throw new Error(
      `Se esperaba el período ${id} del empleado ${empleadoDelEscenario(this).id} en estado "${esperado}" ` +
        `y GET /vacaciones/${id} trae ${extracto(periodo)}`
    );
  }
});

// La confirmación sale de vacaciones.programadas; su texto lleva las fechas del período
// (con el desfase de la zona, "2026-12-16T08:00:00-05:00"), así se reconoce cuál es.
Then('eventualmente el empleado debe recibir la confirmación de sus vacaciones', async function () {
  const empleado = empleadoDelEscenario(this);
  const { fechaInicio, fechaFin } = periodoDelEscenario(this);
  const notificacion = await esperarNotificacion(
    this,
    empleado,
    (n) =>
      n.tipo === 'VACACIONES' &&
      /quedaron programadas/.test(n.mensaje) &&
      n.mensaje.includes(fechaInicio) &&
      n.mensaje.includes(fechaFin),
    `la confirmación de las vacaciones del ${fechaInicio} al ${fechaFin}`
  );
  this.log(`Confirmación: "${notificacion.mensaje}"`);
});

// Validación 3 del Reto 4: el rechazo trae el período con el que choca.
Then('la respuesta debe identificar el período en conflicto', function () {
  const existente = this.recurso('vacaciones existentes');
  const { body, metodo, ruta } = this.ultimaRespuesta;
  const conflicto = body?.periodoEnConflicto;
  if (conflicto?.id !== existente.id || conflicto?.estado !== 'PROGRAMADA') {
    throw new Error(
      `Se esperaba periodoEnConflicto con el período ${existente.id} (PROGRAMADA) en ${metodo} ${ruta}. ` +
        `Cuerpo: ${extracto(body)}`
    );
  }
});

// Un rechazo no deja nada a medias: se cuentan los períodos guardados del empleado.
Then('el empleado no debe tener períodos de vacaciones registrados', async function () {
  const periodos = await periodosDelEmpleado(this);
  if (periodos.length !== 0) {
    throw new Error(`El rechazo no debía guardar ningún período y el empleado tiene: ${extracto(periodos)}`);
  }
});

Then('el empleado debe tener un solo período de vacaciones registrado', async function () {
  const existente = this.recurso('vacaciones existentes');
  const periodos = await periodosDelEmpleado(this);
  if (periodos.length !== 1 || periodos[0].id !== existente.id) {
    throw new Error(`Se esperaba solo el período ${existente.id} y el empleado tiene: ${extracto(periodos)}`);
  }
});

// --- Ciclo completo: suspensión y reactivación (D6, D7) -----------------------

// "De inmediato" = unos segundos después de ahora, en la zona del servicio, y dura un
// poco más que un período del scheduler (ver planDelCiclo). Precondición de D7: antes
// de programar se confirma que el empleado puede iniciar sesión; si no, un 401 después
// no probaría nada.
When('programo un período de vacaciones que inicia de inmediato', async function () {
  const empleado = empleadoDelEscenario(this);
  const antes = await codigoDeLogin(this, empleado);
  if (antes !== 200) {
    throw new Error(`Precondición: el empleado debía poder iniciar sesión antes de las vacaciones y el login respondió ${antes}`);
  }
  const fechas = periodoInmediato(Date.now(), this.configuracion.vacaciones.zonaHoraria, PLAN);
  const respuesta = await programarPeriodo(this, fechas);
  exigirCodigo(respuesta, 201, `Programar las vacaciones del ${fechas.fechaInicio} al ${fechas.fechaFin}`);
  Object.assign(periodoDelEscenario(this), { inicioMs: fechas.inicioMs, finMs: fechas.finMs, programadoEn: Date.now() });
  this.log(
    `Período ${respuesta.body.id}: ${fechas.fechaInicio} → ${fechas.fechaFin} (${this.configuracion.vacaciones.zonaHoraria}); ` +
      `scheduler cada ${this.configuracion.vacaciones.periodoSchedulerS} s, esperas de hasta ` +
      `${PLAN.esperaInicioS} s (inicio) y ${PLAN.esperaFinS} s (fin)`
  );
});

/**
 * Polling de una condición sobre el ciclo: en cada intento se lee el estado del período
 * y el código del login; `diagnostico` decide si ya se cumple o explica por qué no. Si
 * se agota el tiempo, el fallo trae el último diagnóstico (D7).
 */
async function esperarCiclo(mundo, { diagnostico, intentos, descripcion }) {
  const empleado = empleadoDelEscenario(mundo);
  const periodo = periodoDelEscenario(mundo);
  periodo.vistos ??= new Set();
  return eventualmente(
    async () => {
      const { estado } = await consultarPeriodo(mundo, periodo.id);
      periodo.vistos.add(estado);
      const login = await codigoDeLogin(mundo, empleado);
      const resultado = diagnostico({ estado, login, vistos: periodo.vistos });
      if (resultado.cumplida) return { estado, login };
      throw new Error(resultado.motivo);
    },
    { intentos, intervaloMs: mundo.configuracion.polling.intervaloMs, descripcion }
  );
}

function registrarMomento(mundo, que, desdeMs) {
  const periodo = periodoDelEscenario(mundo);
  mundo.log(`${que}: ${((Date.now() - desdeMs) / 1000).toFixed(1)} s después (período ${periodo.id})`);
}

Then('eventualmente el empleado no puede iniciar sesión', ESPERA_DEL_SCHEDULER, async function () {
  const periodo = periodoDelEscenario(this);
  await esperarCiclo(this, {
    diagnostico: diagnosticoSuspension,
    intentos: PLAN.intentosInicio,
    descripcion: `la cuenta de ${empleadoDelEscenario(this).email} se suspende al iniciar el período ${periodo.id}`,
  });
  registrarMomento(this, 'Cuenta suspendida (EN_CURSO y login 401) desde la fechaInicio', periodo.inicioMs);
});

When('el período de vacaciones finaliza', ESPERA_DEL_SCHEDULER, async function () {
  const periodo = periodoDelEscenario(this);
  await eventualmente(
    async () => {
      const { estado } = await consultarPeriodo(this, periodo.id);
      if (estado === 'FINALIZADA') return estado;
      throw new Error(
        `el período ${periodo.id} sigue ${estado} (fechaFin ${periodo.fechaFin}); ` +
          '¿corre el scheduler de vacaciones-service? Revise VACACIONES_CRON y E2E_SCHEDULER_SEGUNDOS'
      );
    },
    {
      intentos: PLAN.intentosFin,
      intervaloMs: this.configuracion.polling.intervaloMs,
      descripcion: `el período ${periodo.id} queda FINALIZADA`,
    }
  );
  registrarMomento(this, 'Período FINALIZADA desde la fechaFin', periodo.finMs);
});

Then('eventualmente el empleado puede iniciar sesión nuevamente', ESPERA_DEL_SCHEDULER, async function () {
  const periodo = periodoDelEscenario(this);
  await esperarCiclo(this, {
    diagnostico: diagnosticoReactivacion,
    intentos: PLAN.intentosFin,
    descripcion: `la cuenta de ${empleadoDelEscenario(this).email} se reactiva al finalizar el período ${periodo.id}`,
  });
  registrarMomento(this, 'Cuenta reactivada (FINALIZADA y login 200) desde la fechaFin', periodo.finMs);
  registrarMomento(this, 'Ciclo completo desde la programación', periodo.programadoEn);
});

// Avisos de cuenta.desactivada (VACACIONES) y cuenta.activada (FIN_VACACIONES), con el
// texto exacto que arma notificaciones-service (Reto 5, D12).
Then('eventualmente el empleado debe recibir el aviso {string}', async function (mensaje) {
  await esperarNotificacion(
    this,
    empleadoDelEscenario(this),
    (n) => n.tipo === 'CUENTA' && n.mensaje === mensaje,
    `el aviso de cuenta "${mensaje}"`
  );
});
