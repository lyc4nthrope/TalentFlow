'use strict';

// Control del tiempo en los escenarios de vacaciones (F9, D6). Funciones puras: se
// prueban con node:test (tests/tiempo.test.js) sin el sistema levantado.

/**
 * "AAAA-MM-DDTHH:mm:ss" del instante en la zona dada, que es el formato con hora que
 * acepta POST /vacaciones (Reto 5, D6: horas locales de ZONA_HORARIA). Se calcula en
 * esa zona y no en la de la máquina que corre la suite (Jenkins puede estar en UTC).
 */
function fechaHoraEnZona(instante, zona) {
  const partes = Object.fromEntries(
    new Intl.DateTimeFormat('en-CA', {
      timeZone: zona,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hourCycle: 'h23',
    })
      .formatToParts(new Date(instante))
      .map(({ type, value }) => [type, value])
  );
  return `${partes.year}-${partes.month}-${partes.day}T${partes.hour}:${partes.minute}:${partes.second}`;
}

// Segundos de adelanto del inicio: alcanzan para que la petición llegue antes de esa
// hora (la regla "no puede estar en el pasado" compara con el reloj del servicio).
const ADELANTO_INICIO_S = 3;
// Lo que el período dura de más sobre un período del scheduler (ver planDelCiclo).
const HOLGURA_DURACION_S = 5;
// Margen para la propagación por el broker y la latencia de las peticiones: los
// eventos llegan en milisegundos (F8), así que es holgado a propósito.
const MARGEN_PROPAGACION_S = 10;

/**
 * Fechas y presupuesto de espera del ciclo de vacaciones, derivados del período del
 * scheduler (E2E_SCHEDULER_SEGUNDOS) y del intervalo de polling. Nada depende de una
 * hora exacta de disparo: los pasos esperan una condición observable y el período del
 * scheduler solo fija el tope de esa espera.
 *
 * - El período dura más que un período del scheduler: así la pasada que lo inicia no
 *   puede también finalizarlo (lo haría si las dos fechas ya pasaron) y la cuenta queda
 *   suspendida al menos un período completo, tiempo de sobra para observar el 401.
 * - La espera del inicio cubre el adelanto, una pasada del scheduler y la propagación.
 * - La espera del fin, contada desde que se vio el inicio, cubre lo que queda del
 *   período, una pasada más y la propagación.
 */
function planDelCiclo({ periodoSchedulerS, intervaloMs }) {
  if (!Number.isInteger(periodoSchedulerS) || periodoSchedulerS <= 0) {
    throw new Error('planDelCiclo: "periodoSchedulerS" debe ser un entero positivo');
  }
  if (!Number.isInteger(intervaloMs) || intervaloMs <= 0) {
    throw new Error('planDelCiclo: "intervaloMs" debe ser un entero positivo');
  }
  const duracionS = periodoSchedulerS + HOLGURA_DURACION_S;
  const esperaInicioS = ADELANTO_INICIO_S + periodoSchedulerS + MARGEN_PROPAGACION_S;
  const esperaFinS = duracionS + periodoSchedulerS + MARGEN_PROPAGACION_S;
  const intentosPara = (segundos) => Math.ceil((segundos * 1000) / intervaloMs) + 1;
  const intentosInicio = intentosPara(esperaInicioS);
  const intentosFin = intentosPara(esperaFinS);
  return Object.freeze({
    adelantoS: ADELANTO_INICIO_S,
    duracionS,
    esperaInicioS,
    esperaFinS,
    intentosInicio,
    intentosFin,
    // Límite de Cucumber para un paso que hace una de esas esperas, con 30 s para las
    // peticiones (el de por defecto de world.js no alcanza con el cron de un minuto).
    timeoutPasoMs: Math.max(intentosInicio, intentosFin) * intervaloMs + 30_000,
  });
}

/**
 * Fechas de un período que empieza "de inmediato": `adelantoS` después del segundo
 * actual y dura `duracionS`. Devuelve las cadenas para la API y los instantes.
 */
function periodoInmediato(ahoraMs, zona, { adelantoS, duracionS }) {
  const inicioMs = Math.floor(ahoraMs / 1000) * 1000 + adelantoS * 1000;
  const finMs = inicioMs + duracionS * 1000;
  return {
    fechaInicio: fechaHoraEnZona(inicioMs, zona),
    fechaFin: fechaHoraEnZona(finMs, zona),
    inicioMs,
    finMs,
  };
}

module.exports = { fechaHoraEnZona, planDelCiclo, periodoInmediato };
