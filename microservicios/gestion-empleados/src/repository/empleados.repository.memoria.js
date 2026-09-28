// Implementación en memoria (tests unitarios). Replica el comportamiento observable
// del repositorio de PostgreSQL, incluidas las condiciones de actualizar/retirar.
function crearRepositorioEmpleadosEnMemoria({ zonaHoraria = "America/Bogota" } = {}) {
  const empleados = new Map();
  const diaEnZona = new Intl.DateTimeFormat("en-CA", { timeZone: zonaHoraria }); // YYYY-MM-DD

  function conCamposDeRetiro(empleado) {
    return { fechaRetiro: null, motivoRetiro: null, ...empleado };
  }

  return {
    guardar(empleado) {
      const guardado = conCamposDeRetiro(empleado);
      empleados.set(empleado.id, guardado);
      return { ...guardado };
    },

    buscarPorId(id) {
      const empleado = empleados.get(id);
      return empleado ? { ...empleado } : null;
    },

    buscarPorEmail(email) {
      const emailBuscado = email.toLowerCase();
      for (const empleado of empleados.values()) {
        if (empleado.email.toLowerCase() === emailBuscado) {
          return { ...empleado };
        }
      }
      return null;
    },

    buscarPorNumeroEmpleado(numeroEmpleado) {
      for (const empleado of empleados.values()) {
        if (empleado.numeroEmpleado === numeroEmpleado) {
          return { ...empleado };
        }
      }
      return null;
    },

    async listar({ estado, desde, hasta } = {}) {
      return Array.from(empleados.values())
        .filter((empleado) => !estado || empleado.estado === estado)
        .filter((empleado) => {
          if (!desde && !hasta) return true;
          if (!empleado.fechaRetiro) return false;
          const dia = diaEnZona.format(new Date(empleado.fechaRetiro));
          return (!desde || dia >= desde) && (!hasta || dia <= hasta);
        })
        .map((empleado) => ({ ...empleado }));
    },

    async listarPendientes() {
      return Array.from(empleados.values())
        .filter((empleado) => empleado.validacionDepartamento === "PENDIENTE")
        .map((empleado) => ({ ...empleado }));
    },

    async actualizarValidacionDepartamento(id, validacionDepartamento) {
      const empleado = empleados.get(id);
      if (empleado) {
        empleados.set(id, { ...empleado, validacionDepartamento });
      }
    },

    async actualizar(id, cambios) {
      const empleado = empleados.get(id);
      if (!empleado || empleado.estado === "RETIRADO") return null;
      const actualizado = { ...empleado, ...cambios };
      empleados.set(id, actualizado);
      return { ...actualizado };
    },

    async retirar(id, { fechaRetiro, motivo }) {
      const empleado = empleados.get(id);
      if (!empleado || empleado.estado === "RETIRADO") return null;
      const retirado = { ...empleado, estado: "RETIRADO", fechaRetiro, motivoRetiro: motivo };
      empleados.set(id, retirado);
      return { ...retirado };
    }
  };
}

module.exports = { crearRepositorioEmpleadosEnMemoria };
