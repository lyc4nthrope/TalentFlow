const { AppError } = require("../errores");
const {
  ESTADOS_EMPLEADO,
  crearEmpleado,
  validarActualizacion,
  validarMotivoRetiro,
  validarFiltrosListado
} = require("../dominio/empleado");
const { instanteUtc } = require("../eventos/envelope");
const {
  TIPOS_EVENTO_EMPLEADO,
  datosEmpleadoCreado,
  datosEmpleadoActualizado,
  datosEmpleadoRetirado
} = require("../eventos/eventos-empleado");

// Publicador nulo para contextos sin broker (pruebas): cumple el mismo contrato.
const PUBLICADOR_NULO = Object.freeze({ publicar: async () => true });

function crearServicioEmpleados(
  repositorio,
  clienteDepartamentos,
  publicador = PUBLICADOR_NULO,
  { reloj = () => new Date() } = {}
) {
  async function verificarEmailDisponible(email, idPropio = null) {
    const existente = await repositorio.buscarPorEmail(email);
    if (existente && existente.id !== idPropio) {
      throw new AppError(`El email ${email} ya está registrado`, 400, [
        { field: "email", message: "Ya está registrado", rejectedValue: email }
      ]);
    }
  }

  // Consulta a departamentos-service (con Circuit Breaker) y traduce la respuesta al
  // estado de validación. Si el departamento no existe de verdad, rechaza con 400.
  async function validarDepartamento(departamentoId) {
    const resultadoDepto = await clienteDepartamentos.existe(departamentoId);
    if (resultadoDepto === "NO_EXISTE") {
      throw new AppError(`El departamento ${departamentoId} no existe`, 400, [
        { field: "departamentoId", message: "No existe", rejectedValue: departamentoId }
      ]);
    }
    // "EXISTE" -> se verificó de verdad, queda ACEPTADO.
    // "PENDIENTE" -> departamentos-service no respondió (circuito abierto); se acepta
    // igual, sin bloquear a RR. HH., y reconciliarPendientes() lo revisará después.
    return resultadoDepto === "PENDIENTE" ? "PENDIENTE" : "ACEPTADO";
  }

  async function buscarExistente(id) {
    const empleado = await repositorio.buscarPorId(id);
    if (!empleado) {
      throw new AppError(`El empleado con id ${id} no existe`, 404);
    }
    return empleado;
  }

  function rechazarSiRetirado(empleado, accion) {
    if (empleado.estado === ESTADOS_EMPLEADO.RETIRADO) {
      throw new AppError(`El empleado ${empleado.id} está RETIRADO y no se puede ${accion}`, 409);
    }
  }

  async function registrar(datos) {
    const empleado = crearEmpleado(datos);

    await verificarEmailDisponible(empleado.email);

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

    empleado.validacionDepartamento = await validarDepartamento(empleado.departamentoId);

    const guardado = await repositorio.guardar(empleado);
    // Después de persistir: si la publicación falla, el registro no se revierte (el
    // publicador deja el error en el log). Se publica también con validación PENDIENTE,
    // porque el empleado sí quedó registrado (Catálogo 3.1: "la operación se persistió").
    await publicador.publicar(TIPOS_EVENTO_EMPLEADO.CREADO, datosEmpleadoCreado(guardado));
    return guardado;
  }

  async function actualizar(id, datos) {
    const actual = await buscarExistente(id);
    rechazarSiRetirado(actual, "modificar");

    const cambios = validarActualizacion(datos, actual);
    await verificarEmailDisponible(cambios.email, id);

    // Solo se consulta a departamentos si el departamento cambió.
    const validacionDepartamento =
      cambios.departamentoId === actual.departamentoId
        ? actual.validacionDepartamento
        : await validarDepartamento(cambios.departamentoId);

    const actualizado = await repositorio.actualizar(id, { ...cambios, validacionDepartamento });
    if (!actualizado) {
      // Lo retiraron entre la lectura y la escritura (DELETE concurrente).
      throw new AppError(`El empleado ${id} está RETIRADO y no se puede modificar`, 409);
    }

    await publicador.publicar(TIPOS_EVENTO_EMPLEADO.ACTUALIZADO, datosEmpleadoActualizado(actualizado));
    return actualizado;
  }

  // Baja lógica: el empleado no se borra; pasa a RETIRADO con fecha y motivo, y queda
  // disponible para auditoría (Reto 00: "Eliminar = marcar como RETIRADO").
  async function retirar(id, { motivo } = {}) {
    const motivoValidado = validarMotivoRetiro(motivo);
    const actual = await buscarExistente(id);
    rechazarSiRetirado(actual, "retirar de nuevo");

    const retirado = await repositorio.retirar(id, {
      fechaRetiro: instanteUtc(reloj()),
      motivo: motivoValidado
    });
    if (!retirado) {
      // Otro DELETE simultáneo ganó la carrera: ese ya publicó el evento.
      throw new AppError(`El empleado ${id} está RETIRADO y no se puede retirar de nuevo`, 409);
    }

    await publicador.publicar(TIPOS_EVENTO_EMPLEADO.RETIRADO, datosEmpleadoRetirado(retirado));
    return retirado;
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
    return buscarExistente(id);
  }

  async function listar(filtros = {}) {
    return repositorio.listar(validarFiltrosListado(filtros));
  }

  return {
    registrar,
    actualizar,
    retirar,
    consultarPorId,
    listar,
    reconciliarPendientes
  };
}

module.exports = { crearServicioEmpleados };
