'use strict';

const { randomBytes } = require('node:crypto');

const esperar = (ms) => new Promise((resolver) => setTimeout(resolver, ms));

// Un valor falso no trae detalle (null, false, 0); para que el mensaje final diga
// qué se vio, la condición puede lanzar un Error que lo describa.
function describir(valor) {
  if (valor instanceof Error) return `error: ${valor.message}`;
  return valor === undefined ? 'undefined' : JSON.stringify(valor);
}

// Polling del PDF (D5): reintenta mientras la condición devuelva un valor falso
// o lance, sin sleeps fijos fuera del intervalo entre intentos. Devuelve el valor
// verdadero para que el paso use lo que encontró (por ejemplo, la notificación).
// Si se agota, el mensaje trae lo último que se vio, para diagnosticar sin
// repetir la corrida.
async function eventualmente(condicion, { intentos, intervaloMs, descripcion = 'condición' } = {}) {
  if (!Number.isInteger(intentos) || intentos <= 0) {
    throw new Error('eventualmente: "intentos" debe ser un entero positivo');
  }
  if (!Number.isInteger(intervaloMs) || intervaloMs < 0) {
    throw new Error('eventualmente: "intervaloMs" debe ser un entero mayor o igual a 0');
  }
  const inicio = Date.now();
  let ultimo;
  for (let intento = 1; intento <= intentos; intento++) {
    try {
      const valor = await condicion();
      if (valor) return valor;
      ultimo = valor;
    } catch (error) {
      ultimo = error;
    }
    if (intento < intentos) await esperar(intervaloMs);
  }
  const transcurrido = Date.now() - inicio;
  throw new Error(
    `No se cumplió "${descripcion}" después de ${intentos} intentos (${transcurrido} ms). ` +
      `Último valor observado: ${describir(ultimo)}`
  );
}

// D3: cada escenario crea sus propios datos; el prefijo de tiempo ordena y el
// sufijo aleatorio evita choques entre escenarios del mismo milisegundo.
// La marca de tiempo va en base 36 (8 caracteres hasta el año 2059): los ids de
// empleados, departamentos y vacaciones son VARCHAR(20) en sus BD, y con los 13
// dígitos decimales "D-<ms>-<hex>" ocupaba 22 y departamentos respondía 500.
function idUnico(prefijo = 'E') {
  return `${prefijo}-${Date.now().toString(36)}-${randomBytes(3).toString('hex')}`;
}

function emailUnico(nombre = 'e2e', dominio = 'empresa.com') {
  return `${idUnico(nombre).toLowerCase()}@${dominio}`;
}

module.exports = { eventualmente, idUnico, emailUnico };
