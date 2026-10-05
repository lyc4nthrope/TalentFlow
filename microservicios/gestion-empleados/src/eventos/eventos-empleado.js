// Cargas útiles de los eventos que publica empleados-service, campo por campo según
// el Catálogo de Eventos (secciones 3.1 a 3.3). No agregar ni renombrar campos: el
// auth-service del Reto 5 y el resto del ecosistema dependen de este contrato.

const TIPOS_EVENTO_EMPLEADO = Object.freeze({
  CREADO: "empleado.creado",
  ACTUALIZADO: "empleado.actualizado",
  RETIRADO: "empleado.retirado"
});

// 3.1 empleado.creado
function datosEmpleadoCreado(empleado) {
  return {
    empleadoId: empleado.id,
    nombre: empleado.nombre,
    apellido: empleado.apellido,
    email: empleado.email,
    numeroEmpleado: empleado.numeroEmpleado,
    cargo: empleado.cargo,
    area: empleado.area,
    departamentoId: empleado.departamentoId,
    fechaIngreso: empleado.fechaIngreso,
    estado: empleado.estado
  };
}

// 3.2 empleado.actualizado
function datosEmpleadoActualizado(empleado) {
  return {
    empleadoId: empleado.id,
    nombre: empleado.nombre,
    apellido: empleado.apellido,
    email: empleado.email,
    cargo: empleado.cargo,
    area: empleado.area,
    departamentoId: empleado.departamentoId
  };
}

// 3.3 empleado.retirado
function datosEmpleadoRetirado(empleado) {
  return {
    empleadoId: empleado.id,
    email: empleado.email,
    fechaRetiro: empleado.fechaRetiro,
    motivo: empleado.motivoRetiro
  };
}

module.exports = {
  TIPOS_EVENTO_EMPLEADO,
  datosEmpleadoCreado,
  datosEmpleadoActualizado,
  datosEmpleadoRetirado
};
