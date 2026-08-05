const { empleado: modeloEmpleado, errores } = require("@talentflow/shared");

const { AppError } = errores;
const { crearEmpleado } = modeloEmpleado;

function crearServicioEmpleados(repositorio) {
  function registrar(datos) {
    const empleado = crearEmpleado(datos);

    const emailYaExiste = repositorio.buscarPorEmail(empleado.email);
    if (emailYaExiste) {
      throw new AppError(`El email ${empleado.email} ya está registrado`, 400);
    }

    const numeroYaExiste = repositorio.buscarPorNumeroEmpleado(empleado.numeroEmpleado);
    if (numeroYaExiste) {
      throw new AppError(`El numeroEmpleado ${empleado.numeroEmpleado} ya está registrado`, 400);
    }

    return repositorio.guardar(empleado);
  }

  function consultarPorId(id) {
    const empleado = repositorio.buscarPorId(id);
    if (!empleado) {
      throw new AppError(`El empleado con id ${id} no existe`, 404);
    }
    return empleado;
  }

  return {
    registrar,
    consultarPorId
  };
}

module.exports = { crearServicioEmpleados };
