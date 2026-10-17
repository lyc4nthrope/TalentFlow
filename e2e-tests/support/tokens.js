'use strict';

// Funciones puras sobre textos y tokens: no hablan con la red, así que se prueban
// con node:test (tests/tokens.test.js) sin el sistema levantado.

const UUID = '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}';

// "…use el token 083bc221-c4a2-4653-b368-5e992f6f0130 (vence …)": el texto del correo.
const TOKEN_COMPLETO = new RegExp(`\\btoken\\s+(${UUID})\\b`, 'i');
// "…use el token ****0130 (vence …)": lo que devuelve GET /notificaciones.
const TOKEN_ENMASCARADO = /\btoken\s+\*+([0-9a-f-]{1,12})\b/i;

// Mailhog entrega el cuerpo tal como viajó: en quoted-printable (RFC 2045) un
// renglón largo se corta con "=" al final (salto suave) y los bytes no ASCII van
// como =XX (la ñ de "contraseña" es =C3=B1). Sin decodificar, el token puede
// quedar partido en dos renglones.
function decodificarQuotedPrintable(texto) {
  const sinSaltosSuaves = String(texto).replace(/=\r?\n/g, '');
  const bytes = [];
  for (let i = 0; i < sinSaltosSuaves.length; i++) {
    const hex = sinSaltosSuaves.slice(i + 1, i + 3);
    if (sinSaltosSuaves[i] === '=' && /^[0-9A-Fa-f]{2}$/.test(hex)) {
      bytes.push(parseInt(hex, 16));
      i += 2;
    } else {
      bytes.push(...Buffer.from(sinSaltosSuaves[i], 'utf8'));
    }
  }
  return Buffer.from(bytes).toString('utf8');
}

/** El token de activación o recuperación completo dentro de un texto, o null. */
function extraerToken(texto) {
  const coincidencia = TOKEN_COMPLETO.exec(String(texto ?? ''));
  return coincidencia ? coincidencia[1].toLowerCase() : null;
}

/** Los caracteres visibles de un token enmascarado ("****0130" → "0130"), o null. */
function extraerSufijoEnmascarado(texto) {
  const coincidencia = TOKEN_ENMASCARADO.exec(String(texto ?? ''));
  return coincidencia ? coincidencia[1].toLowerCase() : null;
}

function partesDeJwt(jwt) {
  const partes = String(jwt).split('.');
  if (partes.length !== 3 || partes.some((parte) => parte === '')) {
    throw new Error(`No es un JWT de tres partes: "${String(jwt).slice(0, 20)}…"`);
  }
  return partes;
}

const base64url = (objeto) => Buffer.from(JSON.stringify(objeto)).toString('base64url');

/**
 * El mismo token con otra firma: cabecera y contenido intactos. Se cambia el primer
 * carácter de la firma y no el último, porque en base64url el último puede llevar
 * bits de relleno y cambiarlo podría no cambiar los bytes de la firma.
 */
function conFirmaAlterada(jwt) {
  const [cabecera, carga, firma] = partesDeJwt(jwt);
  const reemplazo = firma[0] === 'A' ? 'B' : 'A';
  return `${cabecera}.${carga}.${reemplazo}${firma.slice(1)}`;
}

/** El mismo contenido "firmado" con alg=none y sin firma (ataque clásico a JWT). */
function sinFirma(jwt) {
  const [, carga] = partesDeJwt(jwt);
  return `${base64url({ alg: 'none', typ: 'JWT' })}.${carga}.`;
}

/** El contenido (claims) de un JWT sin verificar la firma: solo para mensajes y pruebas. */
function contenidoDeJwt(jwt) {
  const [, carga] = partesDeJwt(jwt);
  return JSON.parse(Buffer.from(carga, 'base64url').toString('utf8'));
}

module.exports = {
  decodificarQuotedPrintable,
  extraerToken,
  extraerSufijoEnmascarado,
  conFirmaAlterada,
  sinFirma,
  contenidoDeJwt,
};
