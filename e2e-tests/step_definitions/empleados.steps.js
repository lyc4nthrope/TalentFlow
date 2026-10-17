'use strict';

const { Given } = require('@cucumber/cucumber');
const { prepararEmpleadoActivo, registrarEmpleado } = require('../support/empleados');

// Precondición de los escenarios de USER, vacaciones y offboarding: un empleado
// nuevo de este escenario con la cuenta activa y una contraseña conocida.
Given('que existe un empleado activo con credenciales establecidas', async function () {
  await prepararEmpleadoActivo(this, { alias: 'empleado' });
});

// Un segundo empleado, solo registrado (no hace falta que active su cuenta).
Given('que existe otro empleado registrado', async function () {
  await registrarEmpleado(this, { alias: 'otro empleado' });
});
