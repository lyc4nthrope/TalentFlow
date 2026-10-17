'use strict';

// Operaciones de negocio con nombre en lenguaje natural. Los pasos
// "Cuando solicito <operación>" las ejecutan con el token de la sesión actual, así
// la misma frase sirve para probar que USER recibe 403 y que ADMIN puede (D8).
// Todas actúan sobre el empleado que preparó el escenario (recurso "empleado").

const { eventualmente, idUnico } = require('./polling');
const { datosDeEmpleado } = require('./empleados');
const { exigirCodigo } = require('./world');

// Fecha local AAAA-MM-DD a `dias` de hoy: lejos en el futuro para que el scheduler
// de vacaciones nunca la toque durante la prueba.
function fechaEnDias(dias) {
  const fecha = new Date(Date.now() + dias * 24 * 60 * 60 * 1000);
  return fecha.toISOString().slice(0, 10);
}

// El perfil lo crea gestion-perfiles al consumir empleado.creado: es asíncrono, así
// que antes de editarlo se espera a que exista (polling, D5).
async function esperarPerfil(mundo, empleadoId) {
  const { intentos, intervaloMs } = mundo.configuracion.polling;
  const token = await mundo.tokenDeAdmin();
  await eventualmente(
    async () => {
      const respuesta = await mundo.enviar('GET', `/perfiles/${encodeURIComponent(empleadoId)}`, { token });
      return exigirCodigo(respuesta, 200, `Esperar el perfil de ${empleadoId}`);
    },
    { intentos, intervaloMs, descripcion: `existe el perfil de ${empleadoId}` }
  );
}

const OPERACIONES = {
  'registrar un empleado nuevo': (mundo) =>
    mundo.peticion('POST', '/empleados', {
      cuerpo: datosDeEmpleado(mundo.recurso('departamento').id),
    }),

  'actualizar los datos de un empleado': (mundo) => {
    const { nombre, apellido, email, area, departamentoId, id } = mundo.recurso('empleado');
    return mundo.peticion('PUT', `/empleados/${encodeURIComponent(id)}`, {
      cuerpo: { nombre, apellido, email, area, departamentoId, cargo: 'Analista Senior' },
    });
  },

  'retirar a un empleado': (mundo) =>
    mundo.peticion('DELETE', `/empleados/${encodeURIComponent(mundo.recurso('empleado').id)}`, {
      cuerpo: { motivo: 'RENUNCIA' },
    }),

  'crear un departamento': (mundo) =>
    mundo.peticion('POST', '/departamentos', {
      cuerpo: { id: idUnico('D'), nombre: `Departamento E2E ${idUnico('n')}` },
    }),

  'programar vacaciones para un empleado': (mundo) =>
    mundo.peticion('POST', '/vacaciones', {
      cuerpo: {
        empleadoId: mundo.recurso('empleado').id,
        fechaInicio: fechaEnDias(400),
        fechaFin: fechaEnDias(404),
      },
    }),

  'editar el perfil de un empleado': async (mundo) => {
    const { id } = mundo.recurso('empleado');
    await esperarPerfil(mundo, id);
    return mundo.peticion('PUT', `/perfiles/${encodeURIComponent(id)}`, { cuerpo: { ciudad: 'Bogotá' } });
  },
};

// Las frases admitidas, para el tipo de parámetro {operación} de Cucumber.
const FRASES_DE_OPERACION = Object.keys(OPERACIONES);

module.exports = { OPERACIONES, FRASES_DE_OPERACION, esperarPerfil, fechaEnDias };
