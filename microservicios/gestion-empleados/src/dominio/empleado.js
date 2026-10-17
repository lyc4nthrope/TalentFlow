const { AppError } = require("../errores");

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
    throw new AppError(
      `Faltan los campos obligatorios: ${faltantes.join(", ")}`,
      400,
      faltantes.map((campo) => ({ field: campo, message: "Es un campo obligatorio" }))
    );
  }
}

function validarEmail(email) {
  const patron = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  if (!patron.test(email)) {
    throw new AppError(`El email "${email}" no tiene un formato válido`, 400, [
      { field: "email", message: "No tiene un formato válido", rejectedValue: email }
    ]);
  }
}

function validarEstado(estado) {
  if (!Object.values(ESTADOS_EMPLEADO).includes(estado)) {
    throw new AppError(`El estado "${estado}" no es válido`, 400, [
      { field: "estado", message: "No es un estado válido", rejectedValue: estado }
    ]);
  }
}

// Reto 4 — motivos admitidos para la baja lógica. El Catálogo de Eventos exige un
// "motivo" en empleado.retirado (solo muestra RENUNCIA como ejemplo); se usa una
// lista cerrada para que los consumidores reciban valores predecibles.
const MOTIVOS_RETIRO = Object.freeze(["RENUNCIA", "DESPIDO", "JUBILACION", "FIN_CONTRATO", "OTRO"]);
const MOTIVO_RETIRO_POR_DEFECTO = "RENUNCIA";

// Campos que PUT /empleados/{id} puede modificar: exactamente los que replica
// empleado.actualizado (Catálogo de Eventos, 3.2). El resto de la representación es
// de solo lectura: id, numeroEmpleado y fechaIngreso no cambian, y el estado solo
// cambia por su propio flujo (DELETE = retiro).
const CAMPOS_EDITABLES = Object.freeze(["nombre", "apellido", "email", "cargo", "area", "departamentoId"]);
const CAMPOS_SOLO_LECTURA = Object.freeze([
  "id",
  "numeroEmpleado",
  "fechaIngreso",
  "estado",
  "validacionDepartamento",
  "fechaRetiro",
  "motivoRetiro"
]);

const FORMATO_FECHA = /^\d{4}-\d{2}-\d{2}$/;

function esFechaValida(valor) {
  if (typeof valor !== "string" || !FORMATO_FECHA.test(valor)) return false;
  const fecha = new Date(`${valor}T00:00:00Z`);
  // Descarta fechas con formato correcto pero inexistentes (2026-02-30).
  return !Number.isNaN(fecha.getTime()) && fecha.toISOString().slice(0, 10) === valor;
}

// Valida el cuerpo de PUT y devuelve solo los campos editables. Evita la asignación
// masiva: un campo desconocido es un error (probablemente un typo del cliente), y un
// campo de solo lectura solo se acepta si no cambia (permite GET → editar → PUT).
function validarActualizacion(datos, empleadoActual) {
  if (datos === null || typeof datos !== "object" || Array.isArray(datos)) {
    throw new AppError("El cuerpo debe ser un objeto JSON", 400);
  }
  const errores = [];

  for (const campo of Object.keys(datos)) {
    if (CAMPOS_EDITABLES.includes(campo)) continue;
    if (CAMPOS_SOLO_LECTURA.includes(campo)) {
      if (datos[campo] !== empleadoActual[campo]) {
        errores.push({ field: campo, message: "No es modificable", rejectedValue: datos[campo] });
      }
      continue;
    }
    errores.push({ field: campo, message: "Campo desconocido", rejectedValue: datos[campo] });
  }

  for (const campo of CAMPOS_EDITABLES) {
    if (typeof datos[campo] !== "string" || datos[campo].trim() === "") {
      errores.push({ field: campo, message: "Es un campo obligatorio" });
    }
  }

  if (errores.length > 0) {
    throw new AppError("La actualización del empleado no es válida", 400, errores);
  }

  validarEmail(datos.email);
  return Object.fromEntries(CAMPOS_EDITABLES.map((campo) => [campo, datos[campo]]));
}

function validarMotivoRetiro(motivo) {
  const valor = motivo ?? MOTIVO_RETIRO_POR_DEFECTO;
  if (!MOTIVOS_RETIRO.includes(valor)) {
    throw new AppError(`El motivo de retiro "${valor}" no es válido`, 400, [
      {
        field: "motivo",
        message: `Debe ser uno de: ${MOTIVOS_RETIRO.join(", ")}`,
        rejectedValue: valor
      }
    ]);
  }
  return valor;
}

// Filtros de GET /empleados. desde/hasta solo tienen sentido sobre la fecha de retiro,
// por eso exigen estado=RETIRADO (evita que un filtro se ignore en silencio).
function validarFiltrosListado({ estado, desde, hasta } = {}) {
  const errores = [];

  if (estado !== undefined && !Object.values(ESTADOS_EMPLEADO).includes(estado)) {
    errores.push({ field: "estado", message: "No es un estado válido", rejectedValue: estado });
  }
  for (const [campo, valor] of [["desde", desde], ["hasta", hasta]]) {
    if (valor !== undefined && !esFechaValida(valor)) {
      errores.push({ field: campo, message: "Debe ser una fecha válida YYYY-MM-DD", rejectedValue: valor });
    }
  }
  if ((desde !== undefined || hasta !== undefined) && estado !== ESTADOS_EMPLEADO.RETIRADO) {
    errores.push({ field: "estado", message: "desde/hasta solo aplican con estado=RETIRADO" });
  }
  if (errores.length === 0 && desde && hasta && desde > hasta) {
    errores.push({ field: "desde", message: "desde no puede ser posterior a hasta", rejectedValue: desde });
  }

  if (errores.length > 0) {
    throw new AppError("Los filtros de la consulta no son válidos", 400, errores);
  }
  return { estado, desde, hasta };
}

function crearEmpleado(datos) {
  const entrada = { ...datos };
  validarCamposObligatorios(entrada);
  validarEmail(entrada.email);

  const estado = entrada.estado ?? ESTADO_INICIAL;
  validarEstado(estado);
  // El retiro es una transición con fecha y motivo (DELETE /empleados/{id}), no un
  // estado con el que se pueda nacer: registrar a alguien ya RETIRADO dejaría un
  // empleado retirado sin fechaRetiro.
  if (estado === ESTADOS_EMPLEADO.RETIRADO) {
    throw new AppError("Un empleado no puede registrarse como RETIRADO; use DELETE /empleados/{id}", 400, [
      { field: "estado", message: "No se puede registrar como RETIRADO", rejectedValue: estado }
    ]);
  }

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
  CAMPOS_EDITABLES,
  MOTIVOS_RETIRO,
  MOTIVO_RETIRO_POR_DEFECTO,
  crearEmpleado,
  validarActualizacion,
  validarMotivoRetiro,
  validarFiltrosListado
};
