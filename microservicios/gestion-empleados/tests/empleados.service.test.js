const { describe, it, beforeEach } = require("node:test");
const assert = require("node:assert/strict");

const { crearServicioEmpleados } = require("../src/services/empleados.service");
const { crearRepositorioEmpleadosEnMemoria } = require("../src/repository/empleados.repository");
const { AppError } = require("@talentflow/shared").errores;

const EMPLEADO_VALIDO = {
  id: "E001",
  nombre: "Juan",
  apellido: "Pérez",
  email: "juan.perez@empresa.com",
  numeroEmpleado: "EMP-2026-001",
  cargo: "Desarrollador Senior",
  area: "Tecnología",
  departamentoId: "IT",
  fechaIngreso: "2026-02-10"
};

describe("Servicio de empleados", () => {
  let repositorio;
  let servicio;

  beforeEach(() => {
    repositorio = crearRepositorioEmpleadosEnMemoria();
    servicio = crearServicioEmpleados(repositorio);
  });

  describe("registrar", () => {
    it("registra un empleado y lo devuelve con el modelo canónico completo", () => {
      const registrado = servicio.registrar(EMPLEADO_VALIDO);

      assert.equal(registrado.id, "E001");
      assert.equal(registrado.nombre, "Juan");
      assert.equal(registrado.apellido, "Pérez");
      assert.equal(registrado.email, "juan.perez@empresa.com");
      assert.equal(registrado.numeroEmpleado, "EMP-2026-001");
      assert.equal(registrado.cargo, "Desarrollador Senior");
      assert.equal(registrado.area, "Tecnología");
      assert.equal(registrado.departamentoId, "IT");
      assert.equal(registrado.fechaIngreso, "2026-02-10");
      assert.equal(registrado.estado, "ACTIVO");
    });

    it("asigna el estado ACTIVO por defecto cuando no se envía", () => {
      const registrado = servicio.registrar(EMPLEADO_VALIDO);
      assert.equal(registrado.estado, "ACTIVO");
    });

    it("permite el estado ACTIVO explícitamente", () => {
      const registrado = servicio.registrar({ ...EMPLEADO_VALIDO, estado: "ACTIVO" });
      assert.equal(registrado.estado, "ACTIVO");
    });

    it("rechaza un email ya registrado con 400", () => {
      servicio.registrar(EMPLEADO_VALIDO);

      assert.throws(
        () => servicio.registrar({ ...EMPLEADO_VALIDO, id: "E002", numeroEmpleado: "EMP-2026-002" }),
        (error) => {
          assert.ok(error instanceof AppError);
          assert.equal(error.codigoEstado, 400);
          assert.match(error.message, /juan\.perez@empresa\.com/);
          return true;
        }
      );
    });

    it("rechaza un numeroEmpleado ya registrado con 400", () => {
      servicio.registrar(EMPLEADO_VALIDO);

      assert.throws(
        () => servicio.registrar({ ...EMPLEADO_VALIDO, id: "E002", email: "otro@empresa.com" }),
        (error) => {
          assert.ok(error instanceof AppError);
          assert.equal(error.codigoEstado, 400);
          assert.match(error.message, /EMP-2026-001/);
          return true;
        }
      );
    });

    it("rechaza un empleado sin campos obligatorios con 400", () => {
      assert.throws(
        () => servicio.registrar({ id: "E003" }),
        (error) => {
          assert.ok(error instanceof AppError);
          assert.equal(error.codigoEstado, 400);
          return true;
        }
      );
    });

    it("rechaza un email con formato inválido con 400", () => {
      assert.throws(
        () => servicio.registrar({ ...EMPLEADO_VALIDO, email: "no-es-un-email" }),
        (error) => {
          assert.ok(error instanceof AppError);
          assert.equal(error.codigoEstado, 400);
          return true;
        }
      );
    });

    it("rechaza un estado no válido con 400", () => {
      assert.throws(
        () => servicio.registrar({ ...EMPLEADO_VALIDO, estado: "DESCONOCIDO" }),
        (error) => {
          assert.ok(error instanceof AppError);
          assert.equal(error.codigoEstado, 400);
          return true;
        }
      );
    });
  });

  describe("consultarPorId", () => {
    it("devuelve el empleado existente", () => {
      servicio.registrar(EMPLEADO_VALIDO);

      const encontrado = servicio.consultarPorId("E001");
      assert.equal(encontrado.id, "E001");
      assert.equal(encontrado.nombre, "Juan");
    });

    it("lanza 404 cuando el empleado no existe", () => {
      assert.throws(
        () => servicio.consultarPorId("E999"),
        (error) => {
          assert.ok(error instanceof AppError);
          assert.equal(error.codigoEstado, 404);
          assert.equal(error.message, "El empleado con id E999 no existe");
          return true;
        }
      );
    });
  });
});
