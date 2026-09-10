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
    }
  };
}

module.exports = { crearRepositorioEmpleadosEnMemoria };