const { AppError } = require("../errores");
const { crearEmpleado } = require("../dominio/empleado");

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

    const resultadoDepto = await clienteDepartamentos.existe(empleado.departamentoId);
    if (resultadoDepto === "NO_EXISTE") {
      throw new AppError(`El departamento ${empleado.departamentoId} no existe`, 400, [
        {
          field: "departamentoId",
          message: "No existe",
          rejectedValue: empleado.departamentoId
        }
      ]);
    }

    // resultadoDepto === "EXISTE" -> se verificó de verdad, queda ACEPTADO.
    // resultadoDepto === "PENDIENTE" -> departamentos-service no respondió (circuito
    // abierto); se registra igual, sin bloquear a RR. HH., y queda pendiente de que
    // reconciliarPendientes() la revise cuando el servicio se restablezca.
    empleado.validacionDepartamento = resultadoDepto === "PENDIENTE" ? "PENDIENTE" : "ACEPTADO";

    return repositorio.guardar(empleado);
  }

  // Se ejecuta cuando departamentos-service se restablece (Circuit Breaker -> CLOSED).
  // Revisa cada empleado que quedó PENDIENTE y lo mueve a ACEPTADO o RECHAZADO según
  // la respuesta real del servicio. Si a mitad de la revisión el circuito vuelve a
  // abrir, ese empleado se deja igual: se reintentará en el próximo cierre.
  async function reconciliarPendientes() {
    const pendientes = await repositorio.listarPendientes();
    const resultados = [];

    for (const empleado of pendientes) {
      const resultadoDepto = await clienteDepartamentos.existe(empleado.departamentoId);

      if (resultadoDepto === "EXISTE") {
        await repositorio.actualizarValidacionDepartamento(empleado.id, "ACEPTADO");
        resultados.push({ id: empleado.id, validacionDepartamento: "ACEPTADO" });
      } else if (resultadoDepto === "NO_EXISTE") {
        await repositorio.actualizarValidacionDepartamento(empleado.id, "RECHAZADO");
        resultados.push({ id: empleado.id, validacionDepartamento: "RECHAZADO" });
      }
    }

    return resultados;
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
    listar,
    reconciliarPendientes
  };
}

module.exports = { crearServicioEmpleados };