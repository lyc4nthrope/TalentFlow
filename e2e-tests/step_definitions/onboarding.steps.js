'use strict';

const { randomUUID } = require('node:crypto');
const { Given, When, Then } = require('@cucumber/cucumber');
const { extracto, exigirCodigo, ALIAS_USUARIO } = require('../support/world');
const { eventualmente, idUnico } = require('../support/polling');
const { extraerToken, contenidoDeJwt } = require('../support/tokens');
const {
  crearDepartamento,
  datosDeEmpleado,
  registrarEmpleado,
  esperarNotificacionDeActivacion,
  esperarCorreoConToken,
  esperarTokenDeActivacion,
  establecerContrasena,
  contrasenaValida,
} = require('../support/empleados');

// Onboarding (P4). El empleado del escenario es siempre el recurso "empleado"; el
// registro intentado (válido o no) queda en "registro intentado" para que los
// Entonces sepan qué se mandó.
const ALIAS_INTENTO = 'registro intentado';

// Deja en el reporte cuánto tardó un efecto asíncrono desde el 201 del alta: son
// las mediciones que calibran el polling (D5).
function registrarDemora(mundo, empleado, efecto) {
  mundo.log(`${efecto} de ${empleado.id}: ${Date.now() - empleado.registradoEn} ms después del alta`);
}

function empleadoDelEscenario(mundo) {
  return mundo.recurso(ALIAS_USUARIO);
}

// El alta como Cuando: con el token de la sesión (ADMIN en los Antecedentes) y la
// respuesta registrada para el Entonces. Solo un 201 deja el recurso "empleado".
async function intentarRegistro(mundo, datos) {
  mundo.recursos.set(ALIAS_INTENTO, datos);
  const respuesta = await mundo.peticion('POST', '/empleados', { cuerpo: datos });
  if (respuesta.status === 201) mundo.recursos.set(ALIAS_USUARIO, { ...datos, registradoEn: Date.now() });
  return respuesta;
}

// --- Precondiciones ----------------------------------------------------------

Given('que existe un departamento para el empleado nuevo', async function () {
  await crearDepartamento(this);
});

Given('que Recursos Humanos registró a un empleado nuevo', async function () {
  await registrarEmpleado(this, { alias: ALIAS_USUARIO });
});

Given('eventualmente el empleado recibe por correo su token de activación', async function () {
  const empleado = empleadoDelEscenario(this);
  await esperarTokenDeActivacion(this, empleado);
  registrarDemora(this, empleado, 'Token de activación en el correo');
});

Given('que el empleado ya estableció su contraseña con el token recibido', async function () {
  const empleado = empleadoDelEscenario(this);
  const password = contrasenaValida();
  exigirCodigo(
    await establecerContrasena(this, { token: empleado.tokenActivacion, password }),
    200,
    `Activar la cuenta de ${empleado.email}`
  );
  empleado.password = password;
});

// --- Registro ----------------------------------------------------------------

When('registro un empleado nuevo en ese departamento', async function () {
  await intentarRegistro(this, datosDeEmpleado(this.recurso('departamento').id));
});

// Un id con el formato correcto que ningún escenario creó.
When('registro un empleado nuevo en un departamento que no existe', async function () {
  await intentarRegistro(this, datosDeEmpleado(idUnico('D')));
});

When('registro un empleado nuevo sin el campo {string}', async function (campo) {
  const datos = datosDeEmpleado(this.recurso('departamento').id);
  if (!(campo in datos)) {
    throw new Error(`El cuerpo del alta no tiene un campo "${campo}" (campos: ${Object.keys(datos).join(', ')})`);
  }
  delete datos[campo];
  await intentarRegistro(this, datos);
});

When('registro un empleado nuevo con el mismo {string} que el otro empleado', async function (campo) {
  const otro = this.recurso('otro empleado');
  if (!(campo in otro)) throw new Error(`El otro empleado no tiene un campo "${campo}"`);
  await intentarRegistro(this, datosDeEmpleado(this.recurso('departamento').id, { [campo]: otro[campo] }));
});

// gestion-empleados responde los errores de validación con `errors: [{field, message}]`.
Then('el error debe señalar el campo {string}', function (campo) {
  const { body, metodo, ruta } = this.ultimaRespuesta;
  const errores = Array.isArray(body?.errors) ? body.errors : [];
  if (!errores.some((error) => error.field === campo)) {
    throw new Error(
      `Se esperaba un error del campo "${campo}" en ${metodo} ${ruta} y los errores señalan: ` +
        `${errores.map((error) => error.field).join(', ') || 'ninguno'}. Cuerpo: ${extracto(body)}`
    );
  }
});

// El rechazo no debe dejar un alta a medias: el id que se intentó no existe.
Then('el empleado no debe haber quedado registrado', async function () {
  const { id } = this.recurso(ALIAS_INTENTO);
  const respuesta = await this.enviar('GET', `/empleados/${encodeURIComponent(id)}`, {
    token: await this.tokenDeAdmin(),
  });
  exigirCodigo(respuesta, 404, `El alta rechazada no debía crear al empleado ${id}`);
});

// --- Efectos asíncronos del alta (polling, D5) --------------------------------

// Ver esperarNotificacionDeActivacion: la notificación sale de usuario.creado, que
// auth publica solo después de guardar la cuenta PENDIENTE_ACTIVACION.
Then(
  'eventualmente el servicio de autenticación debe haber creado su cuenta pendiente de activación',
  async function () {
    const empleado = empleadoDelEscenario(this);
    await esperarNotificacionDeActivacion(this, empleado);
    registrarDemora(this, empleado, 'Cuenta creada (usuario.creado observado)');
  }
);

Then('eventualmente debe existir una notificación de bienvenida para su email', async function () {
  const empleado = empleadoDelEscenario(this);
  const notificacion = await esperarNotificacionDeActivacion(this, empleado);
  registrarDemora(this, empleado, 'Notificación de bienvenida');
  if (!/bienvenid/i.test(notificacion.mensaje)) {
    throw new Error(`La notificación SEGURIDAD de ${empleado.email} no es de bienvenida: "${notificacion.mensaje}"`);
  }
  empleado.notificacionDeBienvenida = notificacion;
});

// El historial lo puede leer cualquier usuario autenticado: el token completo solo
// debe viajar en el correo (Reto 5).
Then('la notificación no debe mostrar el token completo', function () {
  const { notificacionDeBienvenida: notificacion } = empleadoDelEscenario(this);
  if (!notificacion) throw new Error('Falta el paso que espera la notificación de bienvenida');
  if (extraerToken(notificacion.mensaje)) {
    throw new Error(`La notificación expone el token completo: "${notificacion.mensaje}"`);
  }
});

Then('eventualmente debe llegar a su correo el token de activación', async function () {
  const empleado = empleadoDelEscenario(this);
  const { sufijoToken } = empleado.notificacionDeBienvenida ?? (await esperarNotificacionDeActivacion(this, empleado));
  empleado.tokenActivacion = await esperarCorreoConToken(this, empleado, sufijoToken);
  registrarDemora(this, empleado, 'Correo con el token');
});

Then('eventualmente debe existir su perfil con su nombre completo y su email', async function () {
  const empleado = empleadoDelEscenario(this);
  const { intentos, intervaloMs } = this.configuracion.polling;
  const token = await this.tokenDeAdmin();
  const ruta = `/perfiles/${encodeURIComponent(empleado.id)}`;
  const esperado = { nombre: `${empleado.nombre} ${empleado.apellido}`, email: empleado.email };

  await eventualmente(
    async () => {
      const respuesta = await this.enviar('GET', ruta, { token });
      exigirCodigo(respuesta, 200, `Consultar el perfil de ${empleado.id}`);
      const { nombre, email } = respuesta.body ?? {};
      if (nombre === esperado.nombre && email === esperado.email) return respuesta.body;
      throw new Error(`GET ${ruta} trae ${extracto({ nombre, email })} y se esperaba ${extracto(esperado)}`);
    },
    { intentos, intervaloMs, descripcion: `existe el perfil de ${empleado.id} con su nombre y email` }
  );
  registrarDemora(this, empleado, 'Perfil por defecto');
});

// --- Activación y login --------------------------------------------------------

When('el empleado intenta iniciar sesión con una contraseña que cumple la política', async function () {
  const { email } = empleadoDelEscenario(this);
  await this.peticion('POST', '/auth/login', { cuerpo: { email, password: contrasenaValida() }, token: null });
});

When('el empleado establece su contraseña con el token recibido', async function () {
  const empleado = empleadoDelEscenario(this);
  const password = contrasenaValida();
  const respuesta = await establecerContrasena(
    this,
    { token: empleado.tokenActivacion, password },
    { registrar: true }
  );
  if (respuesta.status === 200) empleado.password = password;
});

When('el empleado establece la contraseña {string} con el token recibido', async function (password) {
  const { tokenActivacion } = empleadoDelEscenario(this);
  await establecerContrasena(this, { token: tokenActivacion, password }, { registrar: true });
});

When('el empleado intenta establecer otra contraseña con el mismo token', async function () {
  const { tokenActivacion } = empleadoDelEscenario(this);
  await establecerContrasena(this, { token: tokenActivacion, password: contrasenaValida() }, { registrar: true });
});

// Mismo formato que un token real (UUID), pero generado por la prueba.
When('alguien intenta establecer la contraseña del empleado con un token inventado', async function () {
  await establecerContrasena(this, { token: randomUUID(), password: contrasenaValida() }, { registrar: true });
});

When('el empleado inicia sesión con su nueva contraseña', async function () {
  const { email, password } = empleadoDelEscenario(this);
  if (!password) throw new Error('El empleado todavía no estableció una contraseña en este escenario');
  await this.peticion('POST', '/auth/login', { cuerpo: { email, password }, token: null });
});

Then('el empleado todavía puede iniciar sesión con su primera contraseña', async function () {
  const { email, password } = empleadoDelEscenario(this);
  await this.iniciarSesion({ email, password }, 'el empleado');
});

// Si la contraseña se rechazó, el token no se consumió: todavía activa la cuenta.
Then('el token recibido todavía sirve para activar la cuenta', async function () {
  const empleado = empleadoDelEscenario(this);
  exigirCodigo(
    await establecerContrasena(this, { token: empleado.tokenActivacion, password: contrasenaValida() }),
    200,
    `Activar la cuenta de ${empleado.email} después del rechazo`
  );
});

// Se lee el contenido del JWT sin verificar la firma (la suite no conoce el secreto):
// lo que importa es a quién y con qué rol lo emitió auth-service.
Then('el token de acceso debe tener el rol {string} e identificar al empleado', function (rol) {
  const empleado = empleadoDelEscenario(this);
  const { body } = this.ultimaRespuesta;
  if (!body?.token) throw new Error(`La respuesta del login no trae un token. Cuerpo: ${extracto(body)}`);
  const contenido = contenidoDeJwt(body.token);
  const problemas = [];
  if (contenido.role !== rol) problemas.push(`role es "${contenido.role}" y se esperaba "${rol}"`);
  if (contenido.sub !== empleado.id) problemas.push(`sub es "${contenido.sub}" y se esperaba "${empleado.id}"`);
  if (contenido.email !== empleado.email) problemas.push(`email es "${contenido.email}" y se esperaba "${empleado.email}"`);
  if (problemas.length) throw new Error(`El token de acceso no es el del empleado: ${problemas.join('; ')}`);
});
