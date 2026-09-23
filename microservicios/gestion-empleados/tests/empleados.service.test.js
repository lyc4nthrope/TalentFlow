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

function clienteDepartamentosFalso({ resultado = "EXISTE" } = {}) {
  return { existe: async () => resultado };
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
        clienteDepartamentosFalso({ resultado: "NO_EXISTE" })
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

    it("registra como PENDIENTE (no rechaza) cuando departamentos no responde (Reto 3)", async () => {
      const repositorio = crearRepositorioEmpleadosEnMemoria();
      const servicioConDeptoCaido = crearServicioEmpleados(
        repositorio,
        clienteDepartamentosFalso({ resultado: "PENDIENTE" })
      );

      const registrado = await servicioConDeptoCaido.registrar(EMPLEADO_VALIDO);

      assert.equal(registrado.id, "E001");
      assert.equal(registrado.validacionDepartamento, "PENDIENTE");
    });
  });

  describe("reconciliarPendientes", () => {
    it("acepta un pendiente cuyo departamento sí existe al reconciliar", async () => {
      let existeRespuesta = "PENDIENTE";
      const clienteMutable = { existe: async () => existeRespuesta };
      const repositorio = crearRepositorioEmpleadosEnMemoria();
      const servicioMutable = crearServicioEmpleados(repositorio, clienteMutable);

      await servicioMutable.registrar(EMPLEADO_VALIDO);
      let pendiente = await servicioMutable.consultarPorId("E001");
      assert.equal(pendiente.validacionDepartamento, "PENDIENTE");

      existeRespuesta = "EXISTE"; // el servicio de departamentos se restableció
      const resultados = await servicioMutable.reconciliarPendientes();

      assert.deepEqual(resultados, [{ id: "E001", validacionDepartamento: "ACEPTADO" }]);
      const reconciliado = await servicioMutable.consultarPorId("E001");
      assert.equal(reconciliado.validacionDepartamento, "ACEPTADO");
    });

    it("rechaza un pendiente cuyo departamento no existe al reconciliar", async () => {
      let existeRespuesta = "PENDIENTE";
      const clienteMutable = { existe: async () => existeRespuesta };
      const repositorio = crearRepositorioEmpleadosEnMemoria();
      const servicioMutable = crearServicioEmpleados(repositorio, clienteMutable);

      await servicioMutable.registrar(EMPLEADO_VALIDO);

      existeRespuesta = "NO_EXISTE"; // se restableció, y el departamento no existía
      const resultados = await servicioMutable.reconciliarPendientes();

      assert.deepEqual(resultados, [{ id: "E001", validacionDepartamento: "RECHAZADO" }]);
      const reconciliado = await servicioMutable.consultarPorId("E001");
      assert.equal(reconciliado.validacionDepartamento, "RECHAZADO");
    });

    it("no toca empleados ya ACEPTADOS y no hace nada si no hay pendientes", async () => {
      const servicio2 = crearServicioEmpleados(
        crearRepositorioEmpleadosEnMemoria(),
        clienteDepartamentosFalso({ resultado: "EXISTE" })
      );
      await servicio2.registrar(EMPLEADO_VALIDO);

      const resultados = await servicio2.reconciliarPendientes();
      assert.deepEqual(resultados, []);
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