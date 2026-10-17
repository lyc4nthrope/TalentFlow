'use strict';

// Operaciones de vacaciones contra el sistema desplegado (F9). Las fechas con hora se
// arman en la zona del servicio (support/tiempo.js) y las esperas del scheduler usan
// el plan derivado de E2E_SCHEDULER_SEGUNDOS, nunca un tiempo fijo.

const { eventualmente } = require('./polling');
const { exigirCodigo, extracto } = require('./world');
const { fechaHoraEnZona } = require('./tiempo');

const DIA_MS = 24 * 60 * 60 * 1000;

/**
 * "AAAA-MM-DDTHH:mm:ss" del día que cae a `dias` de hoy (en la zona del servicio), a la
 * hora dada. Para los períodos lejanos que el scheduler nunca toca durante la prueba.
 */
function fechaHoraEnDias(mundo, dias, hora = '08:00:00') {
  const { zonaHoraria } = mundo.configuracion.vacaciones;
  return `${fechaHoraEnZona(Date.now() + dias * DIA_MS, zonaHoraria).slice(0, 10)}T${hora}`;
}

/**
 * POST /vacaciones para el empleado del escenario con el token de la sesión (ADMIN en
 * los Antecedentes). Con `registrar` la respuesta queda para los Entonces. Un 201 deja
 * el período en el recurso `alias`.
 */
async function programarPeriodo(mundo, { fechaInicio, fechaFin }, { alias = 'vacaciones', registrar = true } = {}) {
  const empleado = mundo.recurso('empleado');
  const enviar = registrar ? mundo.peticion.bind(mundo) : mundo.enviar.bind(mundo);
  const respuesta = await enviar('POST', '/vacaciones', {
    cuerpo: { empleadoId: empleado.id, fechaInicio, fechaFin },
  });
  mundo.recursos.set('período solicitado', { fechaInicio, fechaFin });
  if (respuesta.status === 201) mundo.recursos.set(alias, { ...respuesta.body, fechaInicio, fechaFin });
  return respuesta;
}

async function consultarPeriodo(mundo, id) {
  const respuesta = await mundo.enviar('GET', `/vacaciones/${encodeURIComponent(id)}`, {
    token: await mundo.tokenDeAdmin(),
  });
  return exigirCodigo(respuesta, 200, `Consultar el período ${id}`).body;
}

async function periodosDelEmpleado(mundo) {
  const { id } = mundo.recurso('empleado');
  const respuesta = await mundo.enviar('GET', `/vacaciones?empleadoId=${encodeURIComponent(id)}`, {
    token: await mundo.tokenDeAdmin(),
  });
  return exigirCodigo(respuesta, 200, `Listar los períodos de ${id}`).body;
}

// Código del login del empleado con su contraseña (sin lanzar: 200 o 401 son datos).
async function codigoDeLogin(mundo, { email, password }) {
  const respuesta = await mundo.enviar('POST', '/auth/login', { cuerpo: { email, password }, token: null });
  return respuesta.status;
}

/**
 * Espera una notificación del empleado que cumpla `predicado` (GET /notificaciones/{id}
 * con el token del ADMIN). El error de cada intento lista lo que sí hay, para que un
 * tiempo agotado se entienda sin repetir la corrida.
 */
async function esperarNotificacion(mundo, empleado, predicado, descripcion, { intentos, intervaloMs } = mundo.configuracion.polling) {
  const token = await mundo.tokenDeAdmin();
  const ruta = `/notificaciones/${encodeURIComponent(empleado.id)}`;
  return eventualmente(
    async () => {
      const respuesta = exigirCodigo(await mundo.enviar('GET', ruta, { token }), 200, 'Consultar las notificaciones');
      const lista = (Array.isArray(respuesta.body) ? respuesta.body : []).filter(
        (n) => n.destinatario === empleado.email
      );
      const encontrada = lista.find(predicado);
      if (encontrada) return encontrada;
      throw new Error(
        `GET ${ruta} no trae aún esa notificación para ${empleado.email}; hay: ` +
          extracto(lista.map(({ tipo, mensaje }) => ({ tipo, mensaje })), 600)
      );
    },
    { intentos, intervaloMs, descripcion: `${empleado.email} recibe ${descripcion}` }
  );
}

module.exports = {
  fechaHoraEnDias,
  programarPeriodo,
  consultarPeriodo,
  periodosDelEmpleado,
  codigoDeLogin,
  esperarNotificacion,
};
