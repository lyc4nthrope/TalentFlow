const { empleado: modeloEmpleado, errores } = require("@talentflow/shared");

const { AppError } = errores;
const { crearEmpleado } = modeloEmpleado;

function crearServicioEmpleados(repositorio, clienteDepartamentos) {
  async function registrar(datos) {
    const empleado = crearEmpleado(datos);

    const emailYaExiste = await repositorio.buscarPorEmail(empleado.email);
    if (emailYaExiste) {
      throw new AppError(`El email ${empleado.email} ya está registrado`, 400);
    }

    const numeroYaExiste = await repositorio.buscarPorNumeroEmpleado(empleado.numeroEmpleado);
    if (numeroYaExiste) {
      throw new AppError(`El numeroEmpleado ${empleado.numeroEmpleado} ya está registrado`, 400);
    }

    const departamentoExiste = await clienteDepartamentos.existe(empleado.departamentoId);
    if (!departamentoExiste) {
      throw new AppError(`El departamento ${empleado.departamentoId} no existe`, 400);
    }

    return repositorio.guardar(empleado);
  }

  async function consultarPorId(id) {
    const empleado = await repositorio.buscarPorId(id);
    if (!empleado) {
      throw new AppError(`El empleado con id ${id} no existe`, 404);
    }
    return empleado;
  }

  async function listar() {
    return repositorio.listar();
  }

  return {
    registrar,
    consultarPorId,
    listar
  };
}

module.exports = { crearServicioEmpleados };