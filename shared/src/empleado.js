const { AppError } = require("./errores");

const ESTADOS_EMPLEADO = Object.freeze({
  ACTIVO: "ACTIVO",
  EN_VACACIONES: "EN_VACACIONES",
  RETIRADO: "RETIRADO"
});

const ESTADO_INICIAL = ESTADOS_EMPLEADO.ACTIVO;

const CAMPOS_EMPLEADO = Object.freeze([
  "id",
  "nombre",
  "apellido",
  "email",
  "numeroEmpleado",
  "cargo",
  "area",
  "departamentoId",
  "fechaIngreso",
  "estado"
]);

const CAMPOS_OBLIGATORIOS = Object.freeze([
  "id",
  "nombre",
  "apellido",
  "email",
  "numeroEmpleado",
  "cargo",
  "area",
  "departamentoId",
  "fechaIngreso"
]);

function validarCamposObligatorios(datos) {
  const faltantes = CAMPOS_OBLIGATORIOS.filter(
    (campo) => !datos[campo] || String(datos[campo]).trim() === ""
  );
  if (faltantes.length > 0) {
    throw new AppError(`Faltan los campos obligatorios: ${faltantes.join(", ")}`, 400);
  }
}

function validarEmail(email) {
  const patron = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  if (!patron.test(email)) {
    throw new AppError(`El email "${email}" no tiene un formato válido`, 400);
  }
}

function validarEstado(estado) {
  if (!Object.values(ESTADOS_EMPLEADO).includes(estado)) {
    throw new AppError(`El estado "${estado}" no es válido`, 400);
  }
}

function crearEmpleado(datos) {
  const entrada = { ...datos };
  validarCamposObligatorios(entrada);
  validarEmail(entrada.email);

  const estado = entrada.estado ?? ESTADO_INICIAL;
  validarEstado(estado);

  return {
    id: entrada.id,
    nombre: entrada.nombre,
    apellido: entrada.apellido,
    email: entrada.email,
    numeroEmpleado: entrada.numeroEmpleado,
    cargo: entrada.cargo,
    area: entrada.area,
    departamentoId: entrada.departamentoId,
    fechaIngreso: entrada.fechaIngreso,
    estado
  };
}

module.exports = {
  ESTADOS_EMPLEADO,
  ESTADO_INICIAL,
  CAMPOS_EMPLEADO,
  CAMPOS_OBLIGATORIOS,
  crearEmpleado
};
