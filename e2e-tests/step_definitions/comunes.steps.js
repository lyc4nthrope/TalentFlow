'use strict';

const { Given, Then } = require('@cucumber/cucumber');
const { extracto } = require('../support/world');

// Paso genérico: lo reutilizan todas las características.
Then('la respuesta debe tener código {int}', function (esperado) {
  const respuesta = this.ultimaRespuesta;
  if (!respuesta) {
    throw new Error(`Se esperaba código ${esperado}, pero el escenario no hizo ninguna petición`);
  }
  if (respuesta.status !== esperado) {
    throw new Error(
      `Se esperaba código ${esperado} y se obtuvo ${respuesta.status} ` +
        `en ${respuesta.metodo} ${respuesta.ruta}. Cuerpo: ${extracto(respuesta.body)}`
    );
  }
});

// Deja el token del rol como el de la sesión: los pasos siguientes lo usan.
// USER es el empleado activado en "Dado que existe un empleado activo…".
Given('que estoy autenticado como {string}', async function (rol) {
  await this.autenticarComo(rol);
});

Then('el mensaje de error debe ser {string}', function (esperado) {
  const respuesta = this.ultimaRespuesta;
  const obtenido = respuesta?.body?.message;
  if (obtenido !== esperado) {
    throw new Error(
      `Se esperaba el mensaje "${esperado}" y se obtuvo "${obtenido}" ` +
        `en ${respuesta?.metodo} ${respuesta?.ruta}. Cuerpo: ${extracto(respuesta?.body)}`
    );
  }
});
