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

// ---------------------------------------------------------------------------------
// Reto 4: actualización, baja lógica, auditoría y publicación de eventos
// ---------------------------------------------------------------------------------

function crearPublicadorEspia({ exito = true } = {}) {
  const eventos = [];
  return {
    eventos,
    publicar: async (type, data) => {
      eventos.push({ type, data });
      return exito;
    }
  };
}

const CAMBIOS_VALIDOS = {
  nombre: "Juan",
  apellido: "Pérez Gómez",
  email: "juan.perez@empresa.com",
  cargo: "Tech Lead",
  area: "Tecnología",
  departamentoId: "IT"
};

async function rechazaCon(promesa, codigoEstado) {
  await assert.rejects(promesa, (error) => {
    assert.ok(error instanceof AppError);
    assert.equal(error.codigoEstado, codigoEstado);
    return true;
  });
}

describe("Servicio de empleados — Reto 4", () => {
  let repositorio;
  let publicador;
  let servicio;
  const AHORA = new Date("2026-11-30T16:45:00.123Z");

  beforeEach(() => {
    repositorio = crearRepositorioEmpleadosEnMemoria();
    publicador = crearPublicadorEspia();
    servicio = crearServicioEmpleados(repositorio, clienteDepartamentosFalso(), publicador, {
      reloj: () => AHORA
    });
  });

  describe("registrar publica empleado.creado", () => {
    it("después de guardar, con la carga útil del catálogo", async () => {
      await servicio.registrar(EMPLEADO_VALIDO);

      assert.equal(publicador.eventos.length, 1);
      assert.equal(publicador.eventos[0].type, "empleado.creado");
      assert.equal(publicador.eventos[0].data.empleadoId, "E001");
      assert.ok(await repositorio.buscarPorId("E001"), "el empleado quedó persistido");
    });

    it("no publica nada si el registro es inválido", async () => {
      await rechazaCon(servicio.registrar({ id: "E002" }), 400);
      assert.equal(publicador.eventos.length, 0);
    });

    it("si la publicación falla, el empleado queda registrado igual (no se revierte)", async () => {
      const servicioSinBroker = crearServicioEmpleados(
        repositorio,
        clienteDepartamentosFalso(),
        crearPublicadorEspia({ exito: false })
      );

      const registrado = await servicioSinBroker.registrar(EMPLEADO_VALIDO);

      assert.equal(registrado.id, "E001");
      assert.ok(await repositorio.buscarPorId("E001"));
    });

    it("rechaza registrar a alguien directamente como RETIRADO", async () => {
      await rechazaCon(servicio.registrar({ ...EMPLEADO_VALIDO, estado: "RETIRADO" }), 400);
    });
  });

  describe("actualizar (PUT)", () => {
    beforeEach(async () => {
      await servicio.registrar(EMPLEADO_VALIDO);
      publicador.eventos.length = 0;
    });

    it("actualiza los campos editables y publica empleado.actualizado", async () => {
      const actualizado = await servicio.actualizar("E001", CAMBIOS_VALIDOS);

      assert.equal(actualizado.apellido, "Pérez Gómez");
      assert.equal(actualizado.cargo, "Tech Lead");
      assert.deepEqual(publicador.eventos, [
        {
          type: "empleado.actualizado",
          data: { empleadoId: "E001", ...CAMBIOS_VALIDOS }
        }
      ]);
    });

    it("acepta campos de solo lectura si no cambian (GET → editar → PUT)", async () => {
      const actual = await servicio.consultarPorId("E001");
      const actualizado = await servicio.actualizar("E001", { ...actual, cargo: "Tech Lead" });
      assert.equal(actualizado.cargo, "Tech Lead");
    });

    it("rechaza cambiar un campo de solo lectura", async () => {
      await rechazaCon(servicio.actualizar("E001", { ...CAMBIOS_VALIDOS, numeroEmpleado: "OTRO" }), 400);
    });

    it("rechaza campos desconocidos (asignación masiva)", async () => {
      await rechazaCon(servicio.actualizar("E001", { ...CAMBIOS_VALIDOS, salario: 999999 }), 400);
    });

    it("rechaza si falta un campo editable", async () => {
      const { cargo, ...sinCargo } = CAMBIOS_VALIDOS;
      await rechazaCon(servicio.actualizar("E001", sinCargo), 400);
    });

    it("rechaza un email que ya usa otro empleado, pero acepta el propio", async () => {
      await servicio.registrar({
        ...EMPLEADO_VALIDO,
        id: "E002",
        email: "otro@empresa.com",
        numeroEmpleado: "EMP-2026-002"
      });

      await rechazaCon(servicio.actualizar("E001", { ...CAMBIOS_VALIDOS, email: "otro@empresa.com" }), 400);
      const mismo = await servicio.actualizar("E001", CAMBIOS_VALIDOS);
      assert.equal(mismo.email, "juan.perez@empresa.com");
    });

    it("valida el departamento solo si cambió", async () => {
      let consultas = 0;
      const cliente = {
        existe: async () => {
          consultas += 1;
          return "NO_EXISTE";
        }
      };
      const repo = crearRepositorioEmpleadosEnMemoria();
      repo.guardar({ ...EMPLEADO_VALIDO, estado: "ACTIVO", validacionDepartamento: "ACEPTADO" });
      const servicioDepto = crearServicioEmpleados(repo, cliente);

      await servicioDepto.actualizar("E001", CAMBIOS_VALIDOS);
      assert.equal(consultas, 0, "mismo departamento: no se consulta");

      await rechazaCon(servicioDepto.actualizar("E001", { ...CAMBIOS_VALIDOS, departamentoId: "XX" }), 400);
      assert.equal(consultas, 1);
    });

    it("404 si el empleado no existe", async () => {
      await rechazaCon(servicio.actualizar("E999", CAMBIOS_VALIDOS), 404);
    });

    it("409 si el empleado está RETIRADO, sin publicar nada", async () => {
      await servicio.retirar("E001");
      publicador.eventos.length = 0;

      await rechazaCon(servicio.actualizar("E001", CAMBIOS_VALIDOS), 409);
      assert.equal(publicador.eventos.length, 0);
    });
  });

  describe("retirar (DELETE) — baja lógica", () => {
    beforeEach(async () => {
      await servicio.registrar(EMPLEADO_VALIDO);
      publicador.eventos.length = 0;
    });

    it("no borra: cambia a RETIRADO con fechaRetiro (UTC) y motivo por defecto", async () => {
      const retirado = await servicio.retirar("E001");

      assert.equal(retirado.estado, "RETIRADO");
      assert.equal(retirado.fechaRetiro, "2026-11-30T16:45:00Z");
      assert.equal(retirado.motivoRetiro, "RENUNCIA");
      const persistido = await repositorio.buscarPorId("E001");
      assert.equal(persistido.estado, "RETIRADO", "sigue existiendo en la BD");
    });

    it("publica empleado.retirado con la carga útil del catálogo", async () => {
      await servicio.retirar("E001", { motivo: "JUBILACION" });

      assert.deepEqual(publicador.eventos, [
        {
          type: "empleado.retirado",
          data: {
            empleadoId: "E001",
            email: "juan.perez@empresa.com",
            fechaRetiro: "2026-11-30T16:45:00Z",
            motivo: "JUBILACION"
          }
        }
      ]);
    });

    it("rechaza un motivo que no está en la lista", async () => {
      await rechazaCon(servicio.retirar("E001", { motivo: "PORQUE_SI" }), 400);
      assert.equal((await repositorio.buscarPorId("E001")).estado, "ACTIVO");
    });

    it("409 al retirar dos veces, y el evento se publica una sola vez", async () => {
      await servicio.retirar("E001");
      await rechazaCon(servicio.retirar("E001"), 409);
      assert.equal(publicador.eventos.length, 1);
    });

    it("404 si el empleado no existe", async () => {
      await rechazaCon(servicio.retirar("E999"), 404);
    });
  });

  describe("listar con filtros de auditoría", () => {
    it("filtra por estado", async () => {
      await servicio.registrar(EMPLEADO_VALIDO);
      await servicio.registrar({
        ...EMPLEADO_VALIDO,
        id: "E002",
        email: "otro@empresa.com",
        numeroEmpleado: "EMP-2026-002"
      });
      await servicio.retirar("E002");

      const retirados = await servicio.listar({ estado: "RETIRADO" });

      assert.deepEqual(
        retirados.map((e) => e.id),
        ["E002"]
      );
      assert.equal(retirados[0].fechaRetiro, "2026-11-30T16:45:00Z");
    });

    it("filtra el rango por el día del retiro en la zona horaria de Bogotá", async () => {
      // 2026-03-01T02:00Z en UTC es todavía 2026-02-28 a las 21:00 en Bogotá.
      const repo = crearRepositorioEmpleadosEnMemoria({ zonaHoraria: "America/Bogota" });
      const servicioReloj = crearServicioEmpleados(repo, clienteDepartamentosFalso(), undefined, {
        reloj: () => new Date("2026-03-01T02:00:00Z")
      });
      await servicioReloj.registrar(EMPLEADO_VALIDO);
      await servicioReloj.retirar("E001");

      const febrero = await servicioReloj.listar({ estado: "RETIRADO", desde: "2026-02-01", hasta: "2026-02-28" });
      const marzo = await servicioReloj.listar({ estado: "RETIRADO", desde: "2026-03-01", hasta: "2026-03-31" });

      assert.equal(febrero.length, 1);
      assert.equal(marzo.length, 0);
    });

    it("rechaza filtros inválidos con 400", async () => {
      await rechazaCon(servicio.listar({ estado: "INVENTADO" }), 400);
      await rechazaCon(servicio.listar({ estado: "RETIRADO", desde: "2026-02-30" }), 400);
      await rechazaCon(servicio.listar({ estado: "RETIRADO", desde: "2026-06-30", hasta: "2026-01-01" }), 400);
      await rechazaCon(servicio.listar({ desde: "2026-01-01" }), 400);
      await rechazaCon(servicio.listar({ estado: ["RETIRADO", "ACTIVO"] }), 400);
    });
  });
});