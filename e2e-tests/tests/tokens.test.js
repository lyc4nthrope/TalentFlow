'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const {
  decodificarQuotedPrintable,
  extraerToken,
  extraerSufijoEnmascarado,
  conFirmaAlterada,
  sinFirma,
  contenidoDeJwt,
} = require('../support/tokens');

// Cuerpo real de Mailhog (quoted-printable): la ñ va como =C3=B1 y el token queda
// partido por un salto suave ("=" al final del renglón).
const CORREO =
  'Para establecer o recuperar su contrase=C3=B1a use el token 083bc221-c4a2-4=\r\n' +
  '653-b368-5e992f6f0130 (vence 2026-10-18T18:45:19Z). Bienvenido a TalentFlow=\r\n.';

const API = 'Para establecer o recuperar su contraseña use el token ****0130 (vence 2026-10-18T18:45:19Z).';

test('decodifica quoted-printable: saltos suaves y bytes UTF-8', () => {
  assert.equal(
    decodificarQuotedPrintable(CORREO),
    'Para establecer o recuperar su contraseña use el token 083bc221-c4a2-4653-b368-5e992f6f0130 ' +
      '(vence 2026-10-18T18:45:19Z). Bienvenido a TalentFlow.'
  );
  assert.equal(decodificarQuotedPrintable('a=3Db=\nc'), 'a=bc');
  assert.equal(decodificarQuotedPrintable('sin escapes = ni hex =ZZ'), 'sin escapes = ni hex =ZZ');
});

test('extrae el token completo del correo ya decodificado, y no del crudo partido', () => {
  assert.equal(extraerToken(decodificarQuotedPrintable(CORREO)), '083bc221-c4a2-4653-b368-5e992f6f0130');
  assert.equal(extraerToken(CORREO), null);
  assert.equal(extraerToken('use el TOKEN 083BC221-C4A2-4653-B368-5E992F6F0130'), '083bc221-c4a2-4653-b368-5e992f6f0130');
});

test('no confunde el token enmascarado ni textos sin token con uno completo', () => {
  assert.equal(extraerToken(API), null);
  assert.equal(extraerToken('Bienvenido a TalentFlow.'), null);
  assert.equal(extraerToken(undefined), null);
});

test('extrae los caracteres visibles del token enmascarado de la API', () => {
  assert.equal(extraerSufijoEnmascarado(API), '0130');
  assert.equal(extraerSufijoEnmascarado('use el token ****'), null);
  assert.equal(extraerSufijoEnmascarado('Su cuenta fue activada.'), null);
  assert.equal(extraerSufijoEnmascarado(null), null);
});

const b64 = (objeto) => Buffer.from(JSON.stringify(objeto)).toString('base64url');
const JWT = `${b64({ alg: 'HS256', typ: 'JWT' })}.${b64({ sub: 'E-1', role: 'ADMIN' })}.AbCdEfGh`;

test('la firma alterada conserva cabecera y contenido y cambia el primer carácter de la firma', () => {
  const alterado = conFirmaAlterada(JWT);
  const [c1, p1, f1] = JWT.split('.');
  const [c2, p2, f2] = alterado.split('.');
  assert.equal(c2, c1);
  assert.equal(p2, p1);
  assert.equal(f2, `B${f1.slice(1)}`);
  assert.equal(conFirmaAlterada('x.y.Bfirma').split('.')[2], 'Afirma');
});

test('el token sin firma declara alg=none, conserva el contenido y termina en "."', () => {
  const token = sinFirma(JWT);
  const [cabecera, carga, firma] = token.split('.');
  assert.deepEqual(JSON.parse(Buffer.from(cabecera, 'base64url')), { alg: 'none', typ: 'JWT' });
  assert.equal(carga, JWT.split('.')[1]);
  assert.equal(firma, '');
  assert.deepEqual(contenidoDeJwt(JWT), { sub: 'E-1', role: 'ADMIN' });
});

test('rechaza con un mensaje claro algo que no es un JWT de tres partes', () => {
  assert.throws(() => conFirmaAlterada('no-es-jwt'), /No es un JWT de tres partes/);
  assert.throws(() => sinFirma('a..c'), /No es un JWT de tres partes/);
});
