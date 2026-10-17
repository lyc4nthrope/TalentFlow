'use strict';

const { When, Then, defineParameterType } = require('@cucumber/cucumber');
const { extracto, exigirCodigo, ALIAS_USUARIO } = require('../support/world');
const { OPERACIONES, FRASES_DE_OPERACION, esperarPerfil } = require('../support/operaciones');
const { conFirmaAlterada, sinFirma } = require('../support/tokens');

const escaparRegExp = (texto) => texto.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

// {operación}: una de las frases de support/operaciones.js. Una frase desconocida no
// empareja ningún paso y Cucumber la informa como "undefined" (no como un fallo raro).
defineParameterType({
  name: 'operación',
  regexp: new RegExp(FRASES_DE_OPERACION.map(escaparRegExp).join('|')),
  transformer: (frase) => frase,
});

// Credenciales inválidas, nombradas por lo que son. Las que parten de un token real
// del ADMIN prueban que el Gateway valida la firma y el algoritmo, no solo el formato.
const CREDENCIALES_INVALIDAS = {
  'un texto que no es un JWT': async () => 'Bearer esto-no-es-un-jwt',
  'un token con la firma alterada': async (mundo) => `Bearer ${conFirmaAlterada(await mundo.tokenDeAdmin())}`,
  'un token sin firma': async (mundo) => `Bearer ${sinFirma(await mundo.tokenDeAdmin())}`,
  'un token válido con un esquema distinto de Bearer': async (mundo) => `Basic ${await mundo.tokenDeAdmin()}`,
  'una cabecera Bearer sin token': async () => 'Bearer ',
};

const RECURSOS_CONSULTABLES = ['empleados', 'departamentos', 'perfiles', 'vacaciones', 'notificaciones'];

When('consulto la lista de empleados sin token de autenticación', async function () {
  await this.peticion('GET', '/empleados', { token: null });
});

When('consulto la lista de empleados presentando {string}', async function (credencial) {
  const construir = CREDENCIALES_INVALIDAS[credencial];
  if (!construir) {
    throw new Error(
      `Credencial desconocida "${credencial}". Use una de: ${Object.keys(CREDENCIALES_INVALIDAS).join(' | ')}`
    );
  }
  await this.peticion('GET', '/empleados', { autorizacion: await construir(this) });
});

// Una ruta concreta, para las rutas públicas (documentación, salud). Accept */*
// como un navegador: la documentación es HTML.
When('consulto {string} sin token de autenticación', async function (ruta) {
  await this.peticion('GET', ruta, { token: null, acepta: '*/*' });
});

When('consulto la lista de {string}', async function (recurso) {
  if (!RECURSOS_CONSULTABLES.includes(recurso)) {
    throw new Error(`Recurso desconocido "${recurso}". Use uno de: ${RECURSOS_CONSULTABLES.join(', ')}`);
  }
  await this.peticion('GET', `/${recurso}`);
});

When('solicito {operación}', async function (operacion) {
  await OPERACIONES[operacion](this);
});

When('actualizo la ciudad de mi perfil a {string}', async function (ciudad) {
  const { id } = this.recurso(ALIAS_USUARIO);
  await esperarPerfil(this, id);
  await this.peticion('PUT', `/perfiles/${encodeURIComponent(id)}`, { cuerpo: { ciudad } });
});

When('intento actualizar la ciudad del perfil del otro empleado a {string}', async function (ciudad) {
  const { id } = this.recurso('otro empleado');
  await this.peticion('PUT', `/perfiles/${encodeURIComponent(id)}`, { cuerpo: { ciudad } });
});

// Se relee el perfil: el 200 del PUT no basta, el cambio tiene que haber quedado guardado.
Then('mi perfil debe tener la ciudad {string}', async function (esperada) {
  const { id } = this.recurso(ALIAS_USUARIO);
  const respuesta = exigirCodigo(
    await this.enviar('GET', `/perfiles/${encodeURIComponent(id)}`),
    200,
    'Consultar mi perfil'
  );
  if (respuesta.body?.ciudad !== esperada) {
    throw new Error(
      `Se esperaba la ciudad "${esperada}" en GET /perfiles/${id} y se obtuvo "${respuesta.body?.ciudad}". ` +
        `Cuerpo: ${extracto(respuesta.body)}`
    );
  }
});

// Un rechazo de seguridad debe decir solo qué pasó, no por qué falló la validación
// (librería, firma, algoritmo) ni trazas internas que ayuden a un atacante.
const CAMPOS_DE_ERROR = ['error', 'message', 'path'];
const PISTAS_INTERNAS = /stack|trace|exception|jsonwebtoken|signature|algorithm|secret|HS256|at \S+\.js/i;

Then('la respuesta no debe revelar detalles internos', function () {
  const { body, metodo, ruta } = this.ultimaRespuesta;
  const problemas = [];
  if (body === null || typeof body !== 'object') {
    problemas.push('el cuerpo no es un objeto JSON');
  } else {
    const sobrantes = Object.keys(body).filter((campo) => !CAMPOS_DE_ERROR.includes(campo));
    if (sobrantes.length) problemas.push(`trae campos de más: ${sobrantes.join(', ')}`);
  }
  const texto = JSON.stringify(body) ?? '';
  const pista = PISTAS_INTERNAS.exec(texto);
  if (pista) problemas.push(`menciona "${pista[0]}"`);
  if (problemas.length) {
    throw new Error(
      `La respuesta de ${metodo} ${ruta} revela detalles internos (${problemas.join('; ')}). ` +
        `Se esperaba solo ${CAMPOS_DE_ERROR.join(', ')}. Cuerpo: ${extracto(body)}`
    );
  }
});
