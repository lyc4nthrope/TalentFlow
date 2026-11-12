'use strict';

const { When, Then } = require('@cucumber/cucumber');
const { exigirCodigo, extracto } = require('../support/world');
const { idUnico } = require('../support/polling');

// Ruta del departamento que el escenario registró (recurso "departamento").
const rutaDelDepartamento = (mundo) => `/departamentos/${encodeURIComponent(mundo.recurso('departamento').id)}`;

// Con el token de la sesión: así el mismo paso sirve para cualquier rol.
When('registro un departamento nuevo', async function () {
  const datos = { id: idUnico('D'), nombre: `Departamento E2E ${idUnico('n')}` };
  const respuesta = await this.peticion('POST', '/departamentos', { cuerpo: datos });
  if (respuesta.status === 201) this.recursos.set('departamento', { ...datos });
});

When('cambio el nombre del departamento a {string}', async function (nombre) {
  await this.peticion('PUT', rutaDelDepartamento(this), { cuerpo: { nombre } });
});

When('cambio el nombre de un departamento que no existe a {string}', async function (nombre) {
  await this.peticion('PUT', `/departamentos/${encodeURIComponent(idUnico('X'))}`, { cuerpo: { nombre } });
});

When('elimino el departamento', async function () {
  await this.peticion('DELETE', rutaDelDepartamento(this));
});

When('consulto el departamento', async function () {
  await this.peticion('GET', rutaDelDepartamento(this));
});

// Se relee el departamento: el 200 del PUT no basta, el cambio tiene que haber quedado guardado.
Then('el departamento debe tener el nombre {string}', async function (esperado) {
  const respuesta = exigirCodigo(await this.enviar('GET', rutaDelDepartamento(this)), 200, 'Consultar el departamento');
  if (respuesta.body?.nombre !== esperado) {
    throw new Error(
      `Se esperaba el nombre "${esperado}" en GET ${rutaDelDepartamento(this)} y se obtuvo ` +
        `"${respuesta.body?.nombre}". Cuerpo: ${extracto(respuesta.body)}`
    );
  }
});

Then('el mensaje de error debe ser el de un departamento que no existe', function () {
  const esperado = `El departamento con id ${this.recurso('departamento').id} no existe`;
  const obtenido = this.ultimaRespuesta?.body?.message;
  if (obtenido !== esperado) {
    throw new Error(`Se esperaba el mensaje "${esperado}" y se obtuvo "${obtenido}"`);
  }
});
