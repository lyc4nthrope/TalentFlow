const { describe, it, beforeEach } = require("node:test");
const assert = require("node:assert/strict");

const { crearServicioEmpleados } = require("../src/services/empleados.service");
const { crearRepositorioEmpleadosEnMemoria } = require("../src/repository/empleados.repository.memoria");
const { AppError } = require("../src/errores");

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

function clienteDepartamentosFalso({ existe = true } = {}) {
  return { existe: async () => existe };
}

describe("Servicio de empleados", () => {
  let repositorio;
  let servicio;

  beforeEach(() => {
    repositorio = crearRepositorioEmpleadosEnMemoria();
    servicio = crearServicioEmpleados(repositorio, clienteDepartamentosFalso());
  });

  describe("registrar", () => {
    it("registra un empleado y lo devuelve con el modelo canónico completo", async () => {
      const registrado = await servicio.registrar(EMPLEADO_VALIDO);

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

    it("asigna el estado ACTIVO por defecto cuando no se envía", async () => {
      const registrado = await servicio.registrar(EMPLEADO_VALIDO);
      assert.equal(registrado.estado, "ACTIVO");
    });

    it("permite el estado ACTIVO explícitamente", async () => {
      const registrado = await servicio.registrar({ ...EMPLEADO_VALIDO, estado: "ACTIVO" });
      assert.equal(registrado.estado, "ACTIVO");
    });

    it("rechaza un email ya registrado con 400", async () => {
      await servicio.registrar(EMPLEADO_VALIDO);

      await assert.rejects(
        () => servicio.registrar({ ...EMPLEADO_VALIDO, id: "E002", numeroEmpleado: "EMP-2026-002" }),
        (error) => {
          assert.ok(error instanceof AppError);
          assert.equal(error.codigoEstado, 400);
          assert.match(error.message, /juan\.perez@empresa\.com/);
          return true;
        }
      );
    });

    it("rechaza un numeroEmpleado ya registrado con 400", async () => {
      await servicio.registrar(EMPLEADO_VALIDO);

      await assert.rejects(
        () => servicio.registrar({ ...EMPLEADO_VALIDO, id: "E002", email: "otro@empresa.com" }),
        (error) => {
          assert.ok(error instanceof AppError);
          assert.equal(error.codigoEstado, 400);
          assert.match(error.message, /EMP-2026-001/);
          return true;
        }
      );
    });

    it("rechaza un empleado sin campos obligatorios con 400", async () => {
      await assert.rejects(
        () => servicio.registrar({ id: "E003" }),
        (error) => {
          assert.ok(error instanceof AppError);
          assert.equal(error.codigoEstado, 400);
          return true;
        }
      );
    });

    it("rechaza un email con formato inválido con 400", async () => {
      await assert.rejects(
        () => servicio.registrar({ ...EMPLEADO_VALIDO, email: "no-es-un-email" }),
        (error) => {
          assert.ok(error instanceof AppError);
          assert.equal(error.codigoEstado, 400);
          return true;
        }
      );
    });

    it("rechaza un estado no válido con 400", async () => {
      await assert.rejects(
        () => servicio.registrar({ ...EMPLEADO_VALIDO, estado: "DESCONOCIDO" }),
        (error) => {
          assert.ok(error instanceof AppError);
          assert.equal(error.codigoEstado, 400);
          return true;
        }
      );
    });

    it("rechaza un departamento inexistente con 400 (Reto 2)", async () => {
      const servicioSinDepto = crearServicioEmpleados(
        crearRepositorioEmpleadosEnMemoria(),
        clienteDepartamentosFalso({ existe: false })
      );

      await assert.rejects(
        () => servicioSinDepto.registrar(EMPLEADO_VALIDO),
        (error) => {
          assert.ok(error instanceof AppError);
          assert.equal(error.codigoEstado, 400);
          assert.match(error.message, /departamento IT no existe/);
          return true;
        }
      );
    });

    it("propaga el error 503 cuando el servicio de departamentos no responde", async () => {
      const clienteQueFalla = {
        existe: async () => {
          throw new AppError("No fue posible verificar el departamento IT", 503);
        }
      };
      const servicioConFallo = crearServicioEmpleados(
        crearRepositorioEmpleadosEnMemoria(),
        clienteQueFalla
      );

      await assert.rejects(
        () => servicioConFallo.registrar(EMPLEADO_VALIDO),
        (error) => {
          assert.ok(error instanceof AppError);
          assert.equal(error.codigoEstado, 503);
          return true;
        }
      );
    });
  });

  describe("consultarPorId", () => {
    it("devuelve el empleado existente", async () => {
      await servicio.registrar(EMPLEADO_VALIDO);

      const encontrado = await servicio.consultarPorId("E001");
      assert.equal(encontrado.id, "E001");
      assert.equal(encontrado.nombre, "Juan");
    });

    it("lanza 404 cuando el empleado no existe", async () => {
      await assert.rejects(
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

  describe("listar", () => {
    it("devuelve todos los empleados registrados", async () => {
      await servicio.registrar(EMPLEADO_VALIDO);
      await servicio.registrar({
        ...EMPLEADO_VALIDO,
        id: "E002",
        email: "otro@empresa.com",
        numeroEmpleado: "EMP-2026-002"
      });

      const empleados = await servicio.listar();
      assert.equal(empleados.length, 2);
    });

    it("devuelve una lista vacía cuando no hay empleados", async () => {
      const empleados = await servicio.listar();
      assert.deepEqual(empleados, []);
    });
  });
});