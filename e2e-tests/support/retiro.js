'use strict';

// Offboarding (F10). Funciones puras: diagnóstico del polling negativo del retiro (D7),
// ventana de las pruebas de ausencia y revisión de la fecha de retiro de la auditoría.
// Se prueban con node:test (tests/retiro.test.js) sin el sistema levantado.

const { fechaHoraEnZona } = require('./tiempo');

/**
 * El empleado retirado ya no puede iniciar sesión: gestion-empleados lo tiene RETIRADO
 * y el login responde 401. Si no se cumple, el motivo dice qué servicio falló: el retiro
 * no se aplicó en empleados, o empleados lo retiró y auth no desactivó la cuenta.
 *
 * @param {{ estadoEmpleado: string, login: number }} observacion
 * @returns {{ cumplida: true } | { cumplida: false, motivo: string }}
 */
function diagnosticoRetiro({ estadoEmpleado, login }) {
  if (estadoEmpleado === 'RETIRADO' && login === 401) return { cumplida: true };
  if (estadoEmpleado !== 'RETIRADO') {
    return {
      cumplida: false,
      motivo:
        `el retiro no se aplicó en gestion-empleados: el empleado sigue ${estadoEmpleado} (login ${login})` +
        (login === 401 ? '. Además el login ya falla sin que el empleado esté retirado' : ''),
    };
  }
  return {
    cumplida: false,
    motivo:
      login === 200
        ? 'gestion-empleados lo retiró (RETIRADO) pero la cuenta sigue activa: el login responde 200. ' +
          'Falla de auth-service: ¿consumió empleado.retirado y desactivó la cuenta?'
        : `el empleado está RETIRADO pero el login responde ${login} y se esperaba 401`,
  };
}

/**
 * Cuánto vigilar que algo NO pase. Nunca menos que la ventana mínima configurada
 * (E2E_VENTANA_AUSENCIA_MS); si el control positivo del mismo escenario (lo mismo, pero
 * donde sí debe pasar) tardó más, `factor` veces lo que tardó, así una ausencia no se
 * explica porque hoy el sistema está lento. `maximaMs` acota la espera para que el paso
 * no supere el límite de Cucumber; nunca queda por debajo de la mínima.
 */
function ventanaDeAusencia({ minimaMs, maximaMs, latenciaControlMs, factor = 3 }) {
  if (!Number.isInteger(minimaMs) || minimaMs <= 0) {
    throw new Error('ventanaDeAusencia: "minimaMs" debe ser un entero positivo');
  }
  if (latenciaControlMs === undefined) return minimaMs;
  if (!Number.isFinite(latenciaControlMs) || latenciaControlMs < 0) {
    throw new Error('ventanaDeAusencia: "latenciaControlMs" debe ser un número mayor o igual a 0');
  }
  const segunControl = Math.ceil(factor * latenciaControlMs);
  return Math.max(minimaMs, Math.min(maximaMs ?? segunControl, segunControl));
}

// Instante UTC con segundos, como lo devuelve gestion-empleados ("2026-10-17T20:30:05Z").
const INSTANTE_UTC = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z$/;

/**
 * Problemas de la fechaRetiro de un empleado que se retiró entre `desdeMs` y `hastaMs`
 * (antes y después del DELETE). Debe ser un instante UTC dentro de ese intervalo (con
 * `toleranciaMs` por los segundos truncados y el reloj del contenedor) y caer hoy en la
 * zona del negocio, que es el día con el que filtra la auditoría (?desde/?hasta).
 * Devuelve una lista vacía si todo está bien.
 */
function problemasDeFechaRetiro(fechaRetiro, { desdeMs, hastaMs, zona, toleranciaMs = 2000 }) {
  if (typeof fechaRetiro !== 'string' || !INSTANTE_UTC.test(fechaRetiro)) {
    return [`fechaRetiro no es un instante UTC (AAAA-MM-DDTHH:mm:ssZ): ${JSON.stringify(fechaRetiro)}`];
  }
  const instante = Date.parse(fechaRetiro);
  const problemas = [];
  if (instante < desdeMs - toleranciaMs || instante > hastaMs + toleranciaMs) {
    problemas.push(
      `fechaRetiro ${fechaRetiro} no cae en el momento del retiro ` +
        `(${new Date(desdeMs).toISOString()} a ${new Date(hastaMs).toISOString()})`
    );
  }
  const dia = fechaHoraEnZona(instante, zona).slice(0, 10);
  const hoy = [desdeMs, hastaMs].map((ms) => fechaHoraEnZona(ms, zona).slice(0, 10));
  if (!hoy.includes(dia)) {
    problemas.push(`fechaRetiro ${fechaRetiro} es del ${dia} en ${zona} y se esperaba hoy (${hoy[1]})`);
  }
  return problemas;
}

module.exports = { diagnosticoRetiro, ventanaDeAusencia, problemasDeFechaRetiro };
