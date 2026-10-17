'use strict';

// Diagnóstico del polling sobre una condición negativa (F9, D7). Funciones puras: a
// partir de lo que se observó en cada intento (estado del período y código del login)
// dicen si la condición ya se cumplió y, si no, por qué. Así un tiempo agotado distingue
// "todavía no se procesó" de "el sistema está mal". Se prueban en tests/ciclo.test.js.

const SCHEDULER = '¿corre el scheduler de vacaciones-service? Revise VACACIONES_CRON y E2E_SCHEDULER_SEGUNDOS';

/**
 * La cuenta está suspendida por las vacaciones: el período ya está EN_CURSO y el login
 * del empleado responde 401. Un 401 con el período todavía PROGRAMADA no cuenta: no
 * prueba que lo haya causado el inicio de las vacaciones.
 *
 * @param {{ estado: string, login: number, vistos: Set<string> }} observacion
 * @returns {{ cumplida: true } | { cumplida: false, motivo: string }}
 */
function diagnosticoSuspension({ estado, login, vistos }) {
  if (estado === 'EN_CURSO' && login === 401) return { cumplida: true };
  if (estado === 'PROGRAMADA') {
    return {
      cumplida: false,
      motivo:
        `el período nunca empezó: sigue PROGRAMADA (login ${login}). ${SCHEDULER}` +
        (login === 401 ? '. Además el login ya falla sin que el período haya empezado' : ''),
    };
  }
  if (estado === 'EN_CURSO') {
    return {
      cumplida: false,
      motivo:
        login === 200
          ? 'el período empezó (EN_CURSO) pero la cuenta sigue activa: el login responde 200. ' +
            'Falla de auth-service: ¿consumió vacaciones.iniciadas y suspendió la cuenta?'
          : `el período empezó (EN_CURSO) pero el login responde ${login} y se esperaba 401`,
    };
  }
  // Ya terminó (o se canceló): la suspensión ya no se va a poder observar.
  if (vistos.has('EN_CURSO')) {
    return {
      cumplida: false,
      motivo:
        `el período empezó (EN_CURSO) y quedó ${estado} sin que el login respondiera 401: la cuenta siguió ` +
        `activa todo el período (login ${login}). Falla de auth-service: ¿consumió vacaciones.iniciadas?`,
    };
  }
  return {
    cumplida: false,
    motivo:
      `el período quedó ${estado} sin que se observara la cuenta suspendida ` +
      `(estados vistos: ${[...vistos].join(' → ')}; login ${login})`,
  };
}

/**
 * La cuenta volvió a estar activa al terminar las vacaciones: el período está
 * FINALIZADA y el login responde 200.
 */
function diagnosticoReactivacion({ estado, login }) {
  if (estado === 'FINALIZADA' && login === 200) return { cumplida: true };
  if (estado === 'FINALIZADA') {
    return {
      cumplida: false,
      motivo:
        `el período finalizó (FINALIZADA) pero el login sigue respondiendo ${login}. ` +
        'Falla de auth-service: ¿consumió vacaciones.finalizadas y reactivó la cuenta?',
    };
  }
  if (estado === 'EN_CURSO' || estado === 'PROGRAMADA') {
    return {
      cumplida: false,
      motivo: `el período nunca finalizó: sigue ${estado} (login ${login}). ${SCHEDULER}`,
    };
  }
  return { cumplida: false, motivo: `el período quedó ${estado} y se esperaba FINALIZADA (login ${login})` };
}

module.exports = { diagnosticoSuspension, diagnosticoReactivacion };
