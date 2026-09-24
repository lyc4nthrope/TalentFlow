const { ESTADO_PENDIENTE_VALIDACION } = require("../dominio/empleado");

function crearRepositorioEmpleadosEnMemoria() {
  const empleados = new Map();

  return {
    guardar(empleado) {
      empleados.set(empleado.id, empleado);
      return empleado;
    },

    buscarPorId(id) {
      return empleados.get(id) ?? null;
    },

    buscarPorEmail(email) {
      const emailBuscado = email.toLowerCase();
      for (const empleado of empleados.values()) {
        if (empleado.email.toLowerCase() === emailBuscado) {
          return empleado;
        }
      }
      return null;
    },

    buscarPorNumeroEmpleado(numeroEmpleado) {
      for (const empleado of empleados.values()) {
        if (empleado.numeroEmpleado === numeroEmpleado) {
          return empleado;
        }
      }
      return null;
    },

    async listar() {
      return Array.from(empleados.values());
    },

    async buscarPendientesDeValidacion() {
      return Array.from(empleados.values()).filter(
        (empleado) => empleado.estado === ESTADO_PENDIENTE_VALIDACION
      );
    },

    async actualizarEstado(id, nuevoEstado) {
      const empleado = empleados.get(id);
      if (!empleado) return null;
      const actualizado = { ...empleado, estado: nuevoEstado };
      delete actualizado.estadoDeseado;
      empleados.set(id, actualizado);
      return actualizado;
    }
  };
}

module.exports = { crearRepositorioEmpleadosEnMemoria };