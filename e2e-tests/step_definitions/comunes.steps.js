'use strict';

const { Then } = require('@cucumber/cucumber');
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
