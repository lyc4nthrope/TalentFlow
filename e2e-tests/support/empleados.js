'use strict';

// Preparación de datos de negocio (D3): cada escenario crea su departamento y sus
// empleados con ids y emails únicos, a través del Gateway y con el token del ADMIN,
// igual que lo haría RR. HH. Ningún dato pasa de un escenario a otro.
//
// Bloque principal: prepararEmpleadoActivo() deja un empleado con la cuenta ACTIVA y
// una contraseña conocida, que es la precondición de los escenarios de USER, de
// vacaciones y de offboarding (F7 a F10).

const { randomBytes } = require('node:crypto');
const { eventualmente, idUnico, emailUnico } = require('./polling');
const { exigirCodigo, extracto } = require('./world');
const { decodificarQuotedPrintable, extraerToken, extraerSufijoEnmascarado } = require('./tokens');

// Cumple la política del Reto 5 (D14): 8 a 72 caracteres, mayúscula, minúscula y dígito.
function contrasenaValida() {
  return `Clave${randomBytes(4).toString('hex')}A1`;
}

async function crearDepartamento(mundo, { alias = 'departamento' } = {}) {
  const datos = { id: idUnico('D'), nombre: `Departamento E2E ${idUnico('n')}` };
  const respuesta = await mundo.enviar('POST', '/departamentos', {
    cuerpo: datos,
    token: await mundo.tokenDeAdmin(),
  });
  exigirCodigo(respuesta, 201, 'Crear el departamento de prueba');
  mundo.recursos.set(alias, { ...datos });
  return mundo.recursos.get(alias);
}

// Cuerpo válido de POST /empleados (campos obligatorios de gestion-empleados).
function datosDeEmpleado(departamentoId, cambios = {}) {
  const id = idUnico('E');
  return {
    id,
    nombre: 'Prueba',
    apellido: 'Automatizada',
    email: emailUnico('e2e'),
    numeroEmpleado: `N-${id}`,
    cargo: 'Analista',
    area: 'Pruebas',
    departamentoId,
    fechaIngreso: '2026-03-01',
    ...cambios,
  };
}

/**
 * Registra un empleado (solo el alta: la cuenta queda PENDIENTE de activación).
 * Usa el departamento del escenario o crea uno propio si aún no hay.
 * `registradoEn` es el instante de la respuesta 201: desde ahí se miden los efectos
 * asíncronos (D5).
 */
async function registrarEmpleado(mundo, { alias = 'empleado', cambios } = {}) {
  const departamento = mundo.recursos.get('departamento') ?? (await crearDepartamento(mundo));
  const datos = datosDeEmpleado(departamento.id, cambios);
  const respuesta = await mundo.enviar('POST', '/empleados', {
    cuerpo: datos,
    token: await mundo.tokenDeAdmin(),
  });
  exigirCodigo(respuesta, 201, `Registrar al empleado "${alias}"`);
  mundo.recursos.set(alias, { ...datos, registradoEn: Date.now() });
  return mundo.recursos.get(alias);
}

/**
 * Espera la notificación SEGURIDAD de bienvenida del empleado y la devuelve con el
 * sufijo visible del token (`sufijoToken`).
 *
 * Esa notificación es la huella observable de que auth-service creó la cuenta:
 * notificaciones la arma solo a partir de usuario.creado (desde el Reto 5 ya no
 * sale de empleado.creado), y auth publica ese evento después de guardar la cuenta
 * PENDIENTE_ACTIVACION con el token que generó. Auth no tiene una API para consultar
 * cuentas, así que no hay una evidencia HTTP más directa sin consumir el token.
 *
 * El historial de GET /notificaciones devuelve el token enmascarado ("****0130",
 * decisión de seguridad del Reto 5): sirve para probar que el evento llegó y para
 * reconocer el correo correcto, no para activar.
 */
async function esperarNotificacionDeActivacion(mundo, empleado) {
  const { intentos, intervaloMs } = mundo.configuracion.polling;
  const tokenAdmin = await mundo.tokenDeAdmin();
  const ruta = `/notificaciones/${encodeURIComponent(empleado.id)}`;

  return eventualmente(
    async () => {
      const respuesta = await mundo.enviar('GET', ruta, { token: tokenAdmin });
      exigirCodigo(respuesta, 200, 'Consultar las notificaciones del empleado');
      const lista = Array.isArray(respuesta.body) ? respuesta.body : [];
      const delEmpleado = lista.filter((n) => n.destinatario === empleado.email);
      const seguridad = delEmpleado.find((n) => n.tipo === 'SEGURIDAD' && extraerSufijoEnmascarado(n.mensaje));
      if (seguridad) return { ...seguridad, sufijoToken: extraerSufijoEnmascarado(seguridad.mensaje) };
      throw new Error(
        `GET ${ruta} no trae aún una notificación SEGURIDAD con token para ${empleado.email}; ` +
          `notificaciones vistas para ese email: ${extracto(delEmpleado.map(({ tipo, mensaje }) => ({ tipo, mensaje })))}; ` +
          `destinatarios en el historial: ${[...new Set(lista.map((n) => n.destinatario))].join(', ') || 'ninguno'}`
      );
    },
    { intentos, intervaloMs, descripcion: `llega la notificación de activación de ${empleado.email}` }
  );
}

/**
 * Espera en Mailhog el correo del empleado cuyo token termina en `sufijo` (el que
 * mostró la notificación) y devuelve el token completo. Así no se confunde con otro
 * correo del mismo destinatario (por ejemplo, uno de recuperación).
 */
async function esperarCorreoConToken(mundo, empleado, sufijo) {
  const { intentos, intervaloMs } = mundo.configuracion.polling;
  return eventualmente(
    async () => {
      const correos = await correosPara(mundo, empleado.email);
      const tokens = correos.map(extraerToken).filter(Boolean);
      const token = tokens.find((candidato) => candidato.endsWith(sufijo));
      if (token) return token;
      throw new Error(
        `Mailhog tiene ${correos.length} correo(s) para ${empleado.email} y ninguno con un token que termine en ` +
          `"${sufijo}" (tokens vistos: ${tokens.map((t) => `…${t.slice(-4)}`).join(', ') || 'ninguno'})`
      );
    },
    { intentos, intervaloMs, descripcion: `llega el correo de activación de ${empleado.email} a Mailhog` }
  );
}

/**
 * El token de activación completo del empleado (D4, corregida en F7): primero la
 * notificación por el Gateway, después el correo en Mailhog con el mismo sufijo.
 * Lo guarda en el recurso como `tokenActivacion`.
 */
async function esperarTokenDeActivacion(mundo, empleado) {
  const { sufijoToken } = await esperarNotificacionDeActivacion(mundo, empleado);
  empleado.tokenActivacion = await esperarCorreoConToken(mundo, empleado, sufijoToken);
  return empleado.tokenActivacion;
}

// Cuerpos (decodificados) de los correos que Mailhog recibió para ese destinatario.
async function correosPara(mundo, email) {
  const url = `${mundo.configuracion.mailhogUrl}/api/v2/search?kind=to&query=${encodeURIComponent(email)}`;
  let respuesta;
  try {
    respuesta = await fetch(url, { signal: AbortSignal.timeout(10_000) });
  } catch (error) {
    throw new Error(`GET ${url} no obtuvo respuesta: ${error.cause?.code || error.message} (¿MAILHOG_URL?)`);
  }
  if (!respuesta.ok) throw new Error(`GET ${url} devolvió ${respuesta.status}`);
  const { items = [] } = await respuesta.json();
  return items.map(cuerpoDeCorreo);
}

function cuerpoDeCorreo(correo) {
  const partes = [correo.Content, ...(correo.MIME?.Parts ?? [])].filter(Boolean);
  return partes
    .map((parte) => {
      const codificacion = parte.Headers?.['Content-Transfer-Encoding']?.[0] ?? '';
      const cuerpo = parte.Body ?? '';
      return /quoted-printable/i.test(codificacion) ? decodificarQuotedPrintable(cuerpo) : cuerpo;
    })
    .join('\n');
}

/**
 * POST /auth/reset-password sin autenticación, como lo hace el empleado con el
 * token de su correo. Con `registrar` la respuesta queda como la que verifican los
 * Entonces (paso Cuando); sin él, solo prepara datos.
 */
function establecerContrasena(mundo, { token, password }, { registrar = false } = {}) {
  const enviar = registrar ? mundo.peticion.bind(mundo) : mundo.enviar.bind(mundo);
  return enviar('POST', '/auth/reset-password', {
    cuerpo: { token, newPassword: password },
    token: null,
  });
}

/**
 * Deja un empleado listo para actuar como USER: alta → notificación → token de
 * activación → contraseña propia → login. Guarda en `recursos[alias]` los datos del
 * alta más { password, token, tokenActivacion, msActivacion }.
 */
async function prepararEmpleadoActivo(mundo, { alias = 'empleado' } = {}) {
  const inicio = Date.now();
  const empleado = await registrarEmpleado(mundo, { alias });
  const tokenActivacion = await esperarTokenDeActivacion(mundo, empleado);
  const msActivacion = Date.now() - inicio;

  const password = contrasenaValida();
  const activacion = await establecerContrasena(mundo, { token: tokenActivacion, password });
  exigirCodigo(activacion, 200, `Establecer la contraseña de ${empleado.email} con su token de activación`);

  const token = await mundo.iniciarSesion({ email: empleado.email, password }, `el empleado "${alias}"`);
  Object.assign(empleado, { password, token, msActivacion });
  // Queda en el reporte: es la medición que calibra el polling (D5).
  mundo.log(`Empleado ${empleado.id} (${empleado.email}): token de activación disponible en ${msActivacion} ms`);
  return empleado;
}

module.exports = {
  crearDepartamento,
  datosDeEmpleado,
  registrarEmpleado,
  esperarNotificacionDeActivacion,
  esperarCorreoConToken,
  esperarTokenDeActivacion,
  establecerContrasena,
  prepararEmpleadoActivo,
  contrasenaValida,
  correosPara,
};
