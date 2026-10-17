const { AppError } = require("../errores");
const { instanteUtc } = require("../eventos/envelope");

function filaAEmpleado(fila) {
  if (!fila) return null;
  return {
    id: fila.id,
    nombre: fila.nombre,
    apellido: fila.apellido,
    email: fila.email,
    numeroEmpleado: fila.numero_empleado,
    cargo: fila.cargo,
    area: fila.area,
    departamentoId: fila.departamento_id,
    fechaIngreso:
      fila.fecha_ingreso instanceof Date
        ? fila.fecha_ingreso.toISOString().slice(0, 10)
        : fila.fecha_ingreso,
    estado: fila.estado,
    validacionDepartamento: fila.validacion_departamento,
    fechaRetiro: fila.fecha_retiro ? instanteUtc(fila.fecha_retiro) : null,
    motivoRetiro: fila.motivo_retiro ?? null
  };
}

// Violación de restricción UNIQUE (23505): red de seguridad ante condiciones de
// carrera, cuando dos peticiones simultáneas pasaron la verificación previa.
function traducirErrorUnicidad(error, empleado) {
  if (error.code !== "23505") return error;
  if (error.constraint?.includes("email")) {
    return new AppError(`El email ${empleado.email} ya está registrado`, 400, [
      { field: "email", message: "Ya está registrado", rejectedValue: empleado.email }
    ]);
  }
  if (error.constraint?.includes("numero_empleado")) {
    return new AppError(`El numeroEmpleado ${empleado.numeroEmpleado} ya está registrado`, 400, [
      { field: "numeroEmpleado", message: "Ya está registrado", rejectedValue: empleado.numeroEmpleado }
    ]);
  }
  return new AppError(`El empleado con id ${empleado.id} ya existe`, 400, [
    { field: "id", message: "Ya está registrado", rejectedValue: empleado.id }
  ]);
}

function crearRepositorioEmpleadosPostgres(pool, { zonaHoraria = "America/Bogota" } = {}) {
  return {
    async guardar(empleado) {
      const texto = `
        INSERT INTO empleados
          (id, nombre, apellido, email, numero_empleado, cargo, area, departamento_id, fecha_ingreso, estado, validacion_departamento)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        RETURNING *
      `;
      const valores = [
        empleado.id,
        empleado.nombre,
        empleado.apellido,
        empleado.email,
        empleado.numeroEmpleado,
        empleado.cargo,
        empleado.area,
        empleado.departamentoId,
        empleado.fechaIngreso,
        empleado.estado,
        empleado.validacionDepartamento ?? "ACEPTADO"
      ];
      try {
        const resultado = await pool.query(texto, valores);
        return filaAEmpleado(resultado.rows[0]);
      } catch (error) {
        throw traducirErrorUnicidad(error, empleado);
      }
    },

    async buscarPorId(id) {
      const resultado = await pool.query("SELECT * FROM empleados WHERE id = $1", [id]);
      return filaAEmpleado(resultado.rows[0]);
    },

    async buscarPorEmail(email) {
      const resultado = await pool.query(
        "SELECT * FROM empleados WHERE lower(email) = lower($1)",
        [email]
      );
      return filaAEmpleado(resultado.rows[0]);
    },

    async buscarPorNumeroEmpleado(numeroEmpleado) {
      const resultado = await pool.query(
        "SELECT * FROM empleados WHERE numero_empleado = $1",
        [numeroEmpleado]
      );
      return filaAEmpleado(resultado.rows[0]);
    },

    // Filtros ya validados por el dominio. Las condiciones se arman con parámetros
    // ($n), nunca concatenando valores del cliente: no hay inyección SQL posible.
    // desde/hasta se comparan contra el DÍA del retiro en la zona horaria del negocio:
    // un retiro a las 21:00 en Bogotá es el día siguiente en UTC.
    async listar({ estado, desde, hasta } = {}) {
      const condiciones = [];
      const valores = [];
      if (estado) {
        valores.push(estado);
        condiciones.push(`estado = $${valores.length}`);
      }
      if (desde || hasta) {
        valores.push(zonaHoraria);
        const diaRetiro = `(fecha_retiro AT TIME ZONE $${valores.length})::date`;
        if (desde) {
          valores.push(desde);
          condiciones.push(`${diaRetiro} >= $${valores.length}::date`);
        }
        if (hasta) {
          valores.push(hasta);
          condiciones.push(`${diaRetiro} <= $${valores.length}::date`);
        }
      }
      const where = condiciones.length > 0 ? `WHERE ${condiciones.join(" AND ")}` : "";
      const resultado = await pool.query(
        `SELECT * FROM empleados ${where} ORDER BY creado_en ASC`,
        valores
      );
      return resultado.rows.map(filaAEmpleado);
    },

    async listarPendientes() {
      const resultado = await pool.query(
        "SELECT * FROM empleados WHERE validacion_departamento = 'PENDIENTE' ORDER BY creado_en ASC"
      );
      return resultado.rows.map(filaAEmpleado);
    },

    async actualizarValidacionDepartamento(id, validacionDepartamento) {
      await pool.query("UPDATE empleados SET validacion_departamento = $1 WHERE id = $2", [
        validacionDepartamento,
        id
      ]);
    },

    // Devuelve null si el empleado no existe o ya está RETIRADO: la condición va en el
    // propio UPDATE, así una carrera con un DELETE simultáneo no puede editar a un
    // retirado.
    async actualizar(id, cambios) {
      const texto = `
        UPDATE empleados
           SET nombre = $2, apellido = $3, email = $4, cargo = $5, area = $6,
               departamento_id = $7, validacion_departamento = $8
         WHERE id = $1 AND estado <> 'RETIRADO'
        RETURNING *
      `;
      const valores = [
        id,
        cambios.nombre,
        cambios.apellido,
        cambios.email,
        cambios.cargo,
        cambios.area,
        cambios.departamentoId,
        cambios.validacionDepartamento
      ];
      try {
        const resultado = await pool.query(texto, valores);
        return filaAEmpleado(resultado.rows[0]);
      } catch (error) {
        throw traducirErrorUnicidad(error, { ...cambios, id });
      }
    },

    // Baja lógica atómica: solo retira si todavía no está RETIRADO. Si dos DELETE
    // llegan a la vez, solo uno actualiza la fila (y solo uno publica el evento).
    async retirar(id, { fechaRetiro, motivo }) {
      const resultado = await pool.query(
        `UPDATE empleados
            SET estado = 'RETIRADO', fecha_retiro = $2, motivo_retiro = $3
          WHERE id = $1 AND estado <> 'RETIRADO'
         RETURNING *`,
        [id, fechaRetiro, motivo]
      );
      return filaAEmpleado(resultado.rows[0]);
    }
  };
}

module.exports = { crearRepositorioEmpleadosPostgres };
