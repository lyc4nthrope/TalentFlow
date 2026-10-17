'use strict';

const { Given, When, Then } = require('@cucumber/cucumber');
const { exigirCodigo, extracto, ALIAS_USUARIO } = require('../support/world');
const { eventualmente, nuncaDurante, emailUnico } = require('../support/polling');
const { OPERACIONES } = require('../support/operaciones');
const { correosPara, esperarCorreoConToken, establecerContrasena, contrasenaValida } = require('../support/empleados');
const { extraerToken, extraerSufijoEnmascarado } = require('../support/tokens');
const { codigoDeLogin, consultarPeriodo } = require('../support/vacaciones');
const { diagnosticoRetiro, ventanaDeAusencia, problemasDeFechaRetiro } = require('../support/retiro');

// Offboarding (P5, F10). El empleado es el recurso "empleado" de los Antecedentes
// (activo, con contraseña y un login que ya funcionó). El retiro es el mismo DELETE que
// la operación "retirar a un empleado" de F7 (motivo RENUNCIA).

const AVISO_REACTIVACION = 'Bienvenido de regreso. Su cuenta fue reactivada.';

function empleadoDelEscenario(mundo) {
  return mundo.recurso(ALIAS_USUARIO);
}

async function consultarEmpleado(mundo, id) {
  const respuesta = await mundo.enviar('GET', `/empleados/${encodeURIComponent(id)}`, {
    token: await mundo.tokenDeAdmin(),
  });
  return exigirCodigo(respuesta, 200, `Consultar al empleado ${id}`).body;
}

function pedirRecuperacion(mundo, email, { registrar = false } = {}) {
  const enviar = registrar ? mundo.peticion.bind(mundo) : mundo.enviar.bind(mundo);
  return enviar('POST', '/auth/recover-password', { cuerpo: { email }, token: null });
}

// Notificaciones SEGURIDAD (token enmascarado) dirigidas a ese email. La de
// recuperación no lleva empleadoId (catálogo, Reto 5 D20), así que no sale en
// GET /notificaciones/{empleadoId}: se busca en el historial completo por destinatario.
async function sufijosDeSeguridad(mundo, email) {
  const respuesta = exigirCodigo(
    await mundo.enviar('GET', '/notificaciones', { token: await mundo.tokenDeAdmin() }),
    200,
    'Consultar el historial de notificaciones'
  );
  return (Array.isArray(respuesta.body) ? respuesta.body : [])
    .filter((n) => n.destinatario === email && n.tipo === 'SEGURIDAD')
    .map((n) => extraerSufijoEnmascarado(n.mensaje))
    .filter(Boolean);
}

// Tokens que el empleado ya recibió en este escenario (activación y, si la pidió,
// recuperación): cualquier otro que aparezca es nuevo.
function tokensConocidos(empleado) {
  return [empleado.tokenActivacion, empleado.tokenRecuperacion].filter(Boolean);
}

/**
 * Lo que haya llegado con un token nuevo por cualquiera de los dos canales (el
 * historial del Gateway y el correo en Mailhog), o null si no llegó nada.
 */
async function tokenNuevoRecibido(mundo, empleado) {
  const conocidos = tokensConocidos(empleado);
  const esConocido = (sufijo) => conocidos.some((token) => token.endsWith(sufijo));
  const sufijos = (await sufijosDeSeguridad(mundo, empleado.email)).filter((sufijo) => !esConocido(sufijo));
  const correos = (await correosPara(mundo, empleado.email))
    .map(extraerToken)
    .filter((token) => token && !conocidos.includes(token));
  if (!sufijos.length && !correos.length) return null;
  return (
    `a ${empleado.email} le llegó un token nuevo: notificaciones SEGURIDAD con token ` +
    `${sufijos.map((s) => `****${s}`).join(', ') || '(ninguna)'}; correos con token ` +
    `${correos.map((t) => `…${t.slice(-4)}`).join(', ') || '(ninguno)'}`
  );
}

// El DELETE de la operación "retirar a un empleado" (F7) como precondición: debe
// responder 200 y deja el intervalo del retiro para revisar la fechaRetiro.
async function retirar(mundo) {
  const empleado = empleadoDelEscenario(mundo);
  const desdeMs = Date.now();
  const respuesta = await OPERACIONES['retirar a un empleado'](mundo);
  exigirCodigo(respuesta, 200, `Retirar al empleado ${empleado.id}`);
  empleado.retiro = { desdeMs, hastaMs: Date.now() };
  return respuesta;
}

// --- Precondiciones ----------------------------------------------------------

Given('que Recursos Humanos retiró al empleado', async function () {
  await retirar(this);
});

/**
 * Control positivo de la prueba de ausencia: el mismo pedido, para el mismo email y
 * mientras la cuenta está activa, sí produce el token. Se espera como en el onboarding
 * (D4): la notificación por el Gateway y el correo con el mismo sufijo. Lo que tardó
 * queda en `latenciaRecuperacionMs` para dimensionar la ventana de ausencia.
 */
Given('que el empleado pidió recuperar su contraseña y recibió el token por correo', async function () {
  const empleado = empleadoDelEscenario(this);
  const { intentos, intervaloMs } = this.configuracion.polling;
  const conocidos = tokensConocidos(empleado);
  const inicio = Date.now();
  exigirCodigo(await pedirRecuperacion(this, empleado.email), 202, `Pedir la recuperación de ${empleado.email}`);

  const sufijo = await eventualmente(
    async () => {
      const nuevos = (await sufijosDeSeguridad(this, empleado.email)).filter(
        (s) => !conocidos.some((token) => token.endsWith(s))
      );
      if (nuevos.length) return nuevos[0];
      throw new Error(`GET /notificaciones no trae aún una notificación SEGURIDAD con un token nuevo para ${empleado.email}`);
    },
    { intentos, intervaloMs, descripcion: `llega la notificación de recuperación de ${empleado.email}` }
  );
  empleado.tokenRecuperacion = await esperarCorreoConToken(this, empleado, sufijo);
  empleado.latenciaRecuperacionMs = Date.now() - inicio;
  this.log(`Control positivo: token de recuperación de ${empleado.email} en el correo ${empleado.latenciaRecuperacionMs} ms después del 202`);
});

// --- Retiro ------------------------------------------------------------------

When('el empleado retirado solicita recuperar su contraseña', async function () {
  const empleado = empleadoDelEscenario(this);
  await pedirRecuperacion(this, empleado.email, { registrar: true });
  empleado.recuperacionPedidaEn = Date.now();
});

// Se relee al empleado: el 200 del DELETE no basta, tiene que haber quedado guardado.
Then('el empleado debe quedar en estado {string}', async function (esperado) {
  const { id } = empleadoDelEscenario(this);
  const empleado = await consultarEmpleado(this, id);
  if (empleado.estado !== esperado) {
    throw new Error(`Se esperaba al empleado ${id} en estado "${esperado}" y GET /empleados/${id} trae ${extracto(empleado)}`);
  }
});

/**
 * Polling sobre una condición negativa (D7). La precondición es que el login funcionó:
 * los Antecedentes inician sesión con la contraseña del empleado y guardan el token.
 * En cada intento se leen el estado en gestion-empleados y el código del login; solo
 * cuenta un 401 con el empleado RETIRADO. Si se agota, el fallo dice qué servicio falló.
 */
Then('eventualmente el empleado retirado no puede iniciar sesión', async function () {
  const empleado = empleadoDelEscenario(this);
  if (!empleado.token) {
    throw new Error('Precondición: el empleado debía haber iniciado sesión antes del retiro (falta "Dado que existe un empleado activo con credenciales establecidas")');
  }
  const { intentos, intervaloMs } = this.configuracion.polling;
  const inicio = Date.now();
  await eventualmente(
    async () => {
      const { estado } = await consultarEmpleado(this, empleado.id);
      const resultado = diagnosticoRetiro({ estadoEmpleado: estado, login: await codigoDeLogin(this, empleado) });
      if (resultado.cumplida) return true;
      throw new Error(resultado.motivo);
    },
    { intentos, intervaloMs, descripcion: `la cuenta de ${empleado.email} queda bloqueada después del retiro` }
  );
  this.log(`Login bloqueado (RETIRADO y 401) ${Date.now() - inicio} ms después de empezar a esperar`);
});

// D16: recover-password responde igual exista o no la cuenta. Se compara con la
// respuesta a un email que ningún escenario registró.
Then('la respuesta debe ser igual a la que recibe un correo que no está registrado', async function () {
  const obtenida = this.ultimaRespuesta;
  const referencia = await pedirRecuperacion(this, emailUnico('nadie'));
  if (obtenida.status !== referencia.status || JSON.stringify(obtenida.body) !== JSON.stringify(referencia.body)) {
    throw new Error(
      `La respuesta revela que la cuenta existe: para el retirado ${obtenida.status} ${extracto(obtenida.body)} ` +
        `y para un correo no registrado ${referencia.status} ${extracto(referencia.body)}`
    );
  }
});

/**
 * Prueba de ausencia. Se vigila, desde el 202, que no llegue un token nuevo por ningún
 * canal durante una ventana que nunca es menor que E2E_VENTANA_AUSENCIA_MS ni que el
 * triple de lo que tardó el control positivo de este escenario (ver retiro.js).
 */
Then('no debe llegarle ningún token de recuperación nuevo', async function () {
  const empleado = empleadoDelEscenario(this);
  if (empleado.latenciaRecuperacionMs === undefined || !empleado.recuperacionPedidaEn) {
    throw new Error(
      'La ausencia solo prueba algo con un control positivo: el escenario debe incluir ' +
        '"Dado que el empleado pidió recuperar su contraseña y recibió el token por correo" ' +
        'y "Cuando el empleado retirado solicita recuperar su contraseña"'
    );
  }
  const { intentos, intervaloMs } = this.configuracion.polling;
  const ventanaMs = ventanaDeAusencia({
    minimaMs: this.configuracion.ausencia.ventanaMinimaMs,
    maximaMs: intentos * intervaloMs,
    latenciaControlMs: empleado.latenciaRecuperacionMs,
  });
  const yaPasado = Date.now() - empleado.recuperacionPedidaEn;
  const { observaciones } = await nuncaDurante(() => tokenNuevoRecibido(this, empleado), {
    duracionMs: Math.max(1, ventanaMs - yaPasado),
    intervaloMs,
    descripcion: `un token de recuperación nuevo para el retirado ${empleado.email}`,
  });
  this.log(
    `Sin token nuevo durante ${ventanaMs} ms desde el 202 (${observaciones} observaciones; ` +
      `el control positivo tardó ${empleado.latenciaRecuperacionMs} ms)`
  );
});

// D19: el retiro borra el token pendiente, así que el que llegó antes ya no sirve.
Then('el token de recuperación que recibió antes del retiro ya no sirve', async function () {
  const empleado = empleadoDelEscenario(this);
  if (!empleado.tokenRecuperacion) throw new Error('El empleado no pidió una recuperación antes del retiro en este escenario');
  const respuesta = await establecerContrasena(this, { token: empleado.tokenRecuperacion, password: contrasenaValida() });
  if (respuesta.status !== 401 || respuesta.body?.message !== 'token inválido o expirado') {
    throw new Error(
      `El token de recuperación anterior al retiro debía rechazarse con 401 "token inválido o expirado" y ` +
        `POST /auth/reset-password respondió ${respuesta.status}: ${extracto(respuesta.body)}`
    );
  }
});

// --- Auditoría ---------------------------------------------------------------

When('consulto la auditoría de empleados retirados', async function () {
  await this.peticion('GET', '/empleados?estado=RETIRADO');
});

function listaDeAuditoria(mundo) {
  const { body, metodo, ruta } = mundo.ultimaRespuesta;
  if (!Array.isArray(body)) throw new Error(`${metodo} ${ruta} no devolvió una lista. Cuerpo: ${extracto(body)}`);
  const otros = body.filter((e) => e.estado !== 'RETIRADO');
  if (otros.length) {
    throw new Error(`La auditoría de retirados trae empleados en otro estado: ${extracto(otros.map(({ id, estado }) => ({ id, estado })))}`);
  }
  return body;
}

Then('el empleado retirado debe aparecer con la fecha de hoy y el motivo {string}', function (motivo) {
  const empleado = empleadoDelEscenario(this);
  if (!empleado.retiro) throw new Error('El escenario no retiró al empleado ("Dado que Recursos Humanos retiró al empleado")');
  const lista = listaDeAuditoria(this);
  const encontrado = lista.find((e) => e.id === empleado.id);
  if (!encontrado) {
    throw new Error(`El empleado ${empleado.id} no aparece en GET /empleados?estado=RETIRADO (${lista.length} retirados en la lista)`);
  }
  const problemas = problemasDeFechaRetiro(encontrado.fechaRetiro, {
    ...empleado.retiro,
    zona: this.configuracion.vacaciones.zonaHoraria,
  });
  if (encontrado.motivoRetiro !== motivo) problemas.push(`motivoRetiro es "${encontrado.motivoRetiro}" y se esperaba "${motivo}"`);
  if (problemas.length) throw new Error(`El retiro de ${empleado.id} en la auditoría no es el esperado: ${problemas.join('; ')}`);
  this.log(`Auditoría: ${empleado.id} con fechaRetiro ${encontrado.fechaRetiro} (${lista.length} retirados en la lista)`);
});

// Que no aparezca solo prueba algo si el otro empleado existe y sigue activo.
Then('el otro empleado, que sigue activo, no debe aparecer en la auditoría', async function () {
  const otro = this.recurso('otro empleado');
  const actual = await consultarEmpleado(this, otro.id);
  if (actual.estado !== 'ACTIVO') {
    throw new Error(`Precondición: el otro empleado ${otro.id} debía seguir ACTIVO y está ${actual.estado}`);
  }
  if (listaDeAuditoria(this).some((e) => e.id === otro.id)) {
    throw new Error(`El empleado activo ${otro.id} aparece en la auditoría de retirados`);
  }
});

// --- Retiro durante las vacaciones (Reto 5, D19: vacaciones.finalizadas se ignora) ---

/**
 * Prueba de ausencia sobre la reactivación. Precondición: el período ya está FINALIZADA
 * (el paso anterior lo esperó). Durante la ventana el login debe seguir en 401 y no debe
 * llegar el aviso de regreso. El control positivo es suspension.feature: con los mismos
 * pasos, sin retiro, la cuenta se reactiva en ~1,2 s después de la fechaFin.
 */
Then('la cuenta del empleado retirado no se reactiva', async function () {
  const empleado = empleadoDelEscenario(this);
  const periodo = this.recurso('vacaciones');
  const { estado } = await consultarPeriodo(this, periodo.id);
  if (estado !== 'FINALIZADA') {
    throw new Error(`Precondición: el período ${periodo.id} debía estar FINALIZADA y está ${estado}`);
  }
  const token = await this.tokenDeAdmin();
  const ruta = `/notificaciones/${encodeURIComponent(empleado.id)}`;
  const ventanaMs = this.configuracion.ausencia.ventanaMinimaMs;
  const { observaciones } = await nuncaDurante(
    async () => {
      const login = await codigoDeLogin(this, empleado);
      if (login !== 401) {
        return `el login responde ${login} después de finalizar el período: auth-service reactivó una cuenta retirada (¿aplicó vacaciones.finalizadas sobre DESACTIVADA_PERMANENTE?)`;
      }
      const lista = exigirCodigo(await this.enviar('GET', ruta, { token }), 200, 'Consultar las notificaciones').body;
      const aviso = (Array.isArray(lista) ? lista : []).find(
        (n) => n.destinatario === empleado.email && n.tipo === 'CUENTA' && n.mensaje === AVISO_REACTIVACION
      );
      return aviso ? `llegó el aviso "${AVISO_REACTIVACION}" a un empleado retirado` : null;
    },
    {
      duracionMs: ventanaMs,
      intervaloMs: this.configuracion.polling.intervaloMs,
      descripcion: `la reactivación de la cuenta retirada de ${empleado.email}`,
    }
  );
  this.log(`Cuenta retirada sin reactivar durante ${ventanaMs} ms después de FINALIZADA (${observaciones} observaciones)`);
});
