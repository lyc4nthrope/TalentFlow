const { AppError } = require("../errores");

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
    validacionDepartamento: fila.validacion_departamento
  };
}

function crearRepositorioEmpleadosPostgres(pool) {
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
        if (error.code === "23505") {
          // Violación de restricción UNIQUE (red de seguridad ante condiciones de carrera:
          // dos peticiones simultáneas pasaron la verificación previa en el servicio).
          if (error.constraint?.includes("email")) {
            throw new AppError(`El email ${empleado.email} ya está registrado`, 400, [
              { field: "email", message: "Ya está registrado", rejectedValue: empleado.email }
            ]);
          }
          if (error.constraint?.includes("numero_empleado")) {
            throw new AppError(
              `El numeroEmpleado ${empleado.numeroEmpleado} ya está registrado`,
              400,
              [
                {
                  field: "numeroEmpleado",
                  message: "Ya está registrado",
                  rejectedValue: empleado.numeroEmpleado
                }
              ]
            );
          }
          throw new AppError(`El empleado con id ${empleado.id} ya existe`, 400, [
            { field: "id", message: "Ya está registrado", rejectedValue: empleado.id }
          ]);
        }
        throw error;
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

    async listar() {
      const resultado = await pool.query("SELECT * FROM empleados ORDER BY creado_en ASC");
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
    }
  };
}

module.exports = { crearRepositorioEmpleadosPostgres };