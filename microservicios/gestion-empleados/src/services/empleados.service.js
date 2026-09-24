const { AppError } = require("../errores");
const {
  crearEmpleado,
  ESTADO_INICIAL,
  ESTADO_PENDIENTE_VALIDACION
} = require("../dominio/empleado");

function crearServicioEmpleados(repositorio, clienteDepartamentos) {
  async function registrar(datos) {
    const empleado = crearEmpleado(datos);

    const emailYaExiste = await repositorio.buscarPorEmail(empleado.email);
    if (emailYaExiste) {
      throw new AppError(`El email ${empleado.email} ya está registrado`, 400, [
        { field: "email", message: "Ya está registrado", rejectedValue: empleado.email }
      ]);
    }

    const numeroYaExiste = await repositorio.buscarPorNumeroEmpleado(empleado.numeroEmpleado);
    if (numeroYaExiste) {
      throw new AppError(`El numeroEmpleado ${empleado.numeroEmpleado} ya está registrado`, 400, [
        {
          field: "numeroEmpleado",
          message: "Ya está registrado",
          rejectedValue: empleado.numeroEmpleado
        }
      ]);
    }

    const departamentoExiste = await clienteDepartamentos.existe(empleado.departamentoId);

    if (departamentoExiste === false) {
      throw new AppError(`El departamento ${empleado.departamentoId} no existe`, 400, [
        {
          field: "departamentoId",
          message: "No existe",
          rejectedValue: empleado.departamentoId
        }
      ]);
    }

    if (departamentoExiste === null) {
      // Se preserva el estado que el cliente realmente pidió (estadoDeseado) para
      // restaurarlo cuando reconciliarPendientes() confirme el departamento.
      return repositorio.guardar({
        ...empleado,
        estado: ESTADO_PENDIENTE_VALIDACION,
        estadoDeseado: empleado.estado
      });
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

  // Se dispara cuando el Circuit Breaker cierra (departamentos volvió a responder).
  // Revalida cada empleado en PENDIENTE_VALIDACION contra departamentos y, si ya
  // existe, lo pasa a su estadoDeseado original. Si sigue sin existir o el circuito
  // vuelve a abrirse a mitad de la reconciliación, se deja pendiente para el próximo cierre.
  async function reconciliarPendientes() {
    const pendientes = await repositorio.buscarPendientesDeValidacion();
    let reconciliados = 0;

    for (const empleado of pendientes) {
      const existeAhora = await clienteDepartamentos.existe(empleado.departamentoId);
      if (existeAhora === true) {
        await repositorio.actualizarEstado(empleado.id, empleado.estadoDeseado || ESTADO_INICIAL);
        reconciliados += 1;
      }
      // false o null: se deja igual, se reintentará en el próximo cierre del circuito.
    }

    if (pendientes.length > 0) {
      console.info(
        `🔁 Reconciliación: ${reconciliados}/${pendientes.length} empleados pendientes validados`
      );
    }

    return { total: pendientes.length, reconciliados };
  }

  return {
    registrar,
    consultarPorId,
    listar,
    reconciliarPendientes
  };
}

module.exports = { crearServicioEmpleados };