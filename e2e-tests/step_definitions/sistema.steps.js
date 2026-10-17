'use strict';

const { Given } = require('@cucumber/cucumber');

// /health del Gateway es público y agrega el estado de todos los servicios.
Given('que el sistema está desplegado y operativo', async function () {
  await this.peticion('GET', '/health', { token: null });
});
