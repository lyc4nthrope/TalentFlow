const { describe, it, before, after } = require("node:test");
const assert = require("node:assert/strict");

const { crearApp } = require("../src/app");
const { crearServicioEmpleados } = require("../src/services/empleados.service");
const { crearRepositorioEmpleadosEnMemoria } = require("../src/repository/empleados.repository.memoria");

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

// Doble de prueba: simula el servicio de departamentos sin necesitar uno real corriendo.
const clienteDepartamentosFalso = { existe: async () => "EXISTE" };

describe("API de empleados", () => {
  let servidor;
  let baseUrl;

  before(async () => {
    const repositorio = crearRepositorioEmpleadosEnMemoria();
    const servicio = crearServicioEmpleados(repositorio, clienteDepartamentosFalso);
    const app = crearApp(servicio);

    servidor = await new Promise((resolve) => {
      const server = app.listen(0, () => resolve(server));
    });

    const { port } = servidor.address();
    baseUrl = `http://127.0.0.1:${port}`;
  });

  after(() => {
    servidor.close();
  });

  describe("POST /empleados", () => {
    it("registra un empleado y responde 201 con el empleado creado", async () => {
      const respuesta = await fetch(`${baseUrl}/empleados`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(EMPLEADO_VALIDO)
      });

      assert.equal(respuesta.status, 201);
      assert.equal(respuesta.headers.get("location"), "/empleados/E001");
      const cuerpo = await respuesta.json();
      assert.equal(cuerpo.id, "E001");
      assert.equal(cuerpo.nombre, "Juan");
      assert.equal(cuerpo.estado, "ACTIVO");
    });

    it("responde 400 con mensaje descriptivo cuando el email ya está registrado", async () => {
      const respuesta = await fetch(`${baseUrl}/empleados`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...EMPLEADO_VALIDO, id: "E002", numeroEmpleado: "EMP-2026-002" })
      });

      assert.equal(respuesta.status, 400);
      const cuerpo = await respuesta.json();
      assert.equal(cuerpo.status, 400);
      assert.equal(cuerpo.error, "Bad Request");
      assert.match(cuerpo.message, /juan\.perez@empresa\.com/);
      assert.equal(cuerpo.errors[0].field, "email");
    });

    it("responde 400 con mensaje descriptivo cuando el numeroEmpleado ya está registrado", async () => {
      const respuesta = await fetch(`${baseUrl}/empleados`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...EMPLEADO_VALIDO, id: "E003", email: "otro@empresa.com" })
      });

      assert.equal(respuesta.status, 400);
      const cuerpo = await respuesta.json();
      assert.match(cuerpo.message, /EMP-2026-001/);
      assert.equal(cuerpo.errors[0].field, "numeroEmpleado");
    });

    it("responde 400 cuando falta un campo obligatorio", async () => {
      const respuesta = await fetch(`${baseUrl}/empleados`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id: "E004" })
      });

      assert.equal(respuesta.status, 400);
    });

    it("responde 400 con error limpio cuando el cuerpo JSON es inválido", async () => {
      const respuesta = await fetch(`${baseUrl}/empleados`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: "{esto-no-es-json"
      });

      assert.equal(respuesta.status, 400);
      const cuerpo = await respuesta.json();
      assert.equal(cuerpo.message, "Cuerpo JSON inválido");
    });

    it("responde 400 cuando el departamento no existe (Reto 2)", async () => {
      const repositorio = crearRepositorioEmpleadosEnMemoria();
      const servicioSinDepto = crearServicioEmpleados(repositorio, { existe: async () => "NO_EXISTE" });
      const appSinDepto = crearApp(servicioSinDepto);
      const server = await new Promise((resolve) => {
        const s = appSinDepto.listen(0, () => resolve(s));
      });
      const { port } = server.address();

      const respuesta = await fetch(`http://127.0.0.1:${port}/empleados`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...EMPLEADO_VALIDO, id: "E-NODEPTO", email: "nodepto@empresa.com", numeroEmpleado: "EMP-NODEPTO" })
      });

      assert.equal(respuesta.status, 400);
      const cuerpo = await respuesta.json();
      assert.match(cuerpo.message, /departamento IT no existe/);
      assert.equal(cuerpo.errors[0].field, "departamentoId");

      server.close();
    });

    it("responde 201 con validacionDepartamento PENDIENTE cuando departamentos no responde (Reto 3)", async () => {
      const repositorio = crearRepositorioEmpleadosEnMemoria();
      const servicioConDeptoCaido = crearServicioEmpleados(repositorio, {
        existe: async () => "PENDIENTE"
      });
      const appConDeptoCaido = crearApp(servicioConDeptoCaido);
      const server = await new Promise((resolve) => {
        const s = appConDeptoCaido.listen(0, () => resolve(s));
      });
      const { port } = server.address();

      const respuesta = await fetch(`http://127.0.0.1:${port}/empleados`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          ...EMPLEADO_VALIDO,
          id: "E-PENDIENTE",
          email: "pendiente@empresa.com",
          numeroEmpleado: "EMP-PENDIENTE"
        })
      });

      assert.equal(respuesta.status, 201);
      const cuerpo = await respuesta.json();
      assert.equal(cuerpo.validacionDepartamento, "PENDIENTE");

      server.close();
    });
  });

  describe("GET /empleados/circuito-departamentos", () => {
    it("expone el estado actual del circuito hacia departamentos-service", async () => {
      const repositorio = crearRepositorioEmpleadosEnMemoria();
      const servicio = crearServicioEmpleados(repositorio, { existe: async () => "EXISTE" });
      const clienteConEstado = { estadoActual: () => "OPEN" };
      const appConEstado = crearApp(servicio, clienteConEstado);
      const server = await new Promise((resolve) => {
        const s = appConEstado.listen(0, () => resolve(s));
      });
      const { port } = server.address();

      const respuesta = await fetch(`http://127.0.0.1:${port}/empleados/circuito-departamentos`);

      assert.equal(respuesta.status, 200);
      const cuerpo = await respuesta.json();
      assert.equal(cuerpo.dependencia, "departamentos-service");
      assert.equal(cuerpo.estado, "OPEN");

      server.close();
    });
  });

  describe("GET /health", () => {
    async function consultarHealth({ estadoCircuito = "CLOSED", verificarBaseDeDatos }) {
      const servicio = crearServicioEmpleados(crearRepositorioEmpleadosEnMemoria(), clienteDepartamentosFalso);
      const app = crearApp(servicio, { estadoActual: () => estadoCircuito }, { verificarBaseDeDatos });
      const server = await new Promise((resolve) => {
        const s = app.listen(0, () => resolve(s));
      });
      try {
        const respuesta = await fetch(`http://127.0.0.1:${server.address().port}/health`);
        return { status: respuesta.status, cuerpo: await respuesta.json() };
      } finally {
        server.close();
      }
    }

    it("responde 200 UP con la BD y el estado del circuito", async () => {
      const { status, cuerpo } = await consultarHealth({ verificarBaseDeDatos: async () => {} });

      assert.equal(status, 200);
      assert.equal(cuerpo.status, "UP");
      assert.equal(cuerpo.service, "empleados-service");
      assert.deepEqual(cuerpo.components, {
        app: "UP",
        db: "UP",
        circuitoDepartamentos: "CLOSED",
        broker: "DESCONOCIDO"
      });
    });

    it("responde 503 DOWN cuando la base de datos falla", async () => {
      const { status, cuerpo } = await consultarHealth({
        verificarBaseDeDatos: async () => {
          throw new Error("connection refused");
        }
      });

      assert.equal(status, 503);
      assert.equal(cuerpo.status, "DOWN");
      assert.equal(cuerpo.components.db, "DOWN");
    });

    it("responde 503 DOWN cuando la base de datos no contesta dentro del plazo", async () => {
      const { status, cuerpo } = await consultarHealth({
        verificarBaseDeDatos: () => new Promise(() => {})
      });

      assert.equal(status, 503);
      assert.equal(cuerpo.components.db, "DOWN");
    });

    it("sigue UP con el circuito OPEN: degradado, no caído", async () => {
      const { status, cuerpo } = await consultarHealth({
        estadoCircuito: "OPEN",
        verificarBaseDeDatos: async () => {}
      });

      assert.equal(status, 200);
      assert.equal(cuerpo.status, "UP");
      assert.equal(cuerpo.components.circuitoDepartamentos, "OPEN");
    });
  });

  describe("GET /empleados", () => {
    it("devuelve la lista de empleados registrados con 200", async () => {
      const respuesta = await fetch(`${baseUrl}/empleados`);

      assert.equal(respuesta.status, 200);
      const cuerpo = await respuesta.json();
      assert.ok(Array.isArray(cuerpo));
      assert.ok(cuerpo.some((e) => e.id === "E001"));
    });
  });

  describe("GET /empleados/:id", () => {
    it("devuelve el empleado existente con 200", async () => {
      const respuesta = await fetch(`${baseUrl}/empleados/E001`);

      assert.equal(respuesta.status, 200);
      const cuerpo = await respuesta.json();
      assert.equal(cuerpo.id, "E001");
      assert.equal(cuerpo.nombre, "Juan");
    });

    it("responde 404 con el mensaje exacto cuando el empleado no existe", async () => {
      const respuesta = await fetch(`${baseUrl}/empleados/E999`);

      assert.equal(respuesta.status, 404);
      const cuerpo = await respuesta.json();
      assert.equal(cuerpo.error, "Not Found");
      assert.equal(cuerpo.message, "El empleado con id E999 no existe");
    });
  });

  describe("Rutas no soportadas", () => {
    it("responde 404 con 'Recurso no encontrado' para una ruta desconocida", async () => {
      const respuesta = await fetch(`${baseUrl}/otra-ruta`);

      assert.equal(respuesta.status, 404);
      const cuerpo = await respuesta.json();
      assert.equal(cuerpo.message, "Recurso no encontrado");
      assert.equal(cuerpo.path, "/otra-ruta");
    });

    it("responde 404 con 'Recurso no encontrado' para un método no soportado", async () => {
      const respuesta = await fetch(`${baseUrl}/empleados/E001`, {
        method: "PATCH"
      });

      assert.equal(respuesta.status, 404);
      const cuerpo = await respuesta.json();
      assert.equal(cuerpo.message, "Recurso no encontrado");
    });
  });
});
describe("API de empleados — Reto 4", () => {
  let servidor;
  let baseUrl;

  before(async () => {
    const repositorio = crearRepositorioEmpleadosEnMemoria();
    const servicio = crearServicioEmpleados(repositorio, clienteDepartamentosFalso);
    servidor = await new Promise((resolve) => {
      const s = crearApp(servicio).listen(0, () => resolve(s));
    });
    baseUrl = `http://127.0.0.1:${servidor.address().port}`;

    for (const [id, email, numero] of [
      ["E001", "juan.perez@empresa.com", "EMP-2026-001"],
      ["E002", "ana@empresa.com", "EMP-2026-002"]
    ]) {
      await fetch(`${baseUrl}/empleados`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...EMPLEADO_VALIDO, id, email, numeroEmpleado: numero })
      });
    }
  });

  after(() => servidor.close());

  const enviar = (metodo, ruta, cuerpo) =>
    fetch(`${baseUrl}${ruta}`, {
      method: metodo,
      headers: { "Content-Type": "application/json" },
      body: cuerpo === undefined ? undefined : JSON.stringify(cuerpo)
    });

  it("PUT /empleados/:id responde 200 con el empleado actualizado", async () => {
    const respuesta = await enviar("PUT", "/empleados/E001", {
      nombre: "Juan",
      apellido: "Pérez Gómez",
      email: "juan.perez@empresa.com",
      cargo: "Tech Lead",
      area: "Tecnología",
      departamentoId: "IT"
    });

    assert.equal(respuesta.status, 200);
    const cuerpo = await respuesta.json();
    assert.equal(cuerpo.cargo, "Tech Lead");
  });

  it("PUT con un campo desconocido responde 400 con el detalle en errors", async () => {
    const respuesta = await enviar("PUT", "/empleados/E001", { salario: 1 });

    assert.equal(respuesta.status, 400);
    const cuerpo = await respuesta.json();
    assert.ok(cuerpo.errors.some((e) => e.field === "salario"));
  });

  it("DELETE /empleados/:id con motivo responde 200 con la baja lógica", async () => {
    const respuesta = await enviar("DELETE", "/empleados/E002", { motivo: "DESPIDO" });

    assert.equal(respuesta.status, 200);
    const cuerpo = await respuesta.json();
    assert.equal(cuerpo.estado, "RETIRADO");
    assert.equal(cuerpo.motivoRetiro, "DESPIDO");
    assert.match(cuerpo.fechaRetiro, /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/);
  });

  it("el empleado retirado sigue consultable (no se borró)", async () => {
    const respuesta = await fetch(`${baseUrl}/empleados/E002`);
    assert.equal(respuesta.status, 200);
    assert.equal((await respuesta.json()).estado, "RETIRADO");
  });

  it("DELETE sobre un retirado responde 409 Conflict con el formato de error", async () => {
    const respuesta = await enviar("DELETE", "/empleados/E002");

    assert.equal(respuesta.status, 409);
    const cuerpo = await respuesta.json();
    assert.equal(cuerpo.status, 409);
    assert.equal(cuerpo.error, "Conflict");
  });

  it("GET /empleados?estado=RETIRADO lista solo los retirados, con fechaRetiro", async () => {
    const respuesta = await fetch(`${baseUrl}/empleados?estado=RETIRADO`);

    assert.equal(respuesta.status, 200);
    const cuerpo = await respuesta.json();
    assert.deepEqual(cuerpo.map((e) => e.id), ["E002"]);
    assert.ok(cuerpo[0].fechaRetiro);
  });

  it("GET con rango de fechas incluye al retirado de hoy y excluye otro rango", async () => {
    const hoy = new Intl.DateTimeFormat("en-CA", { timeZone: "America/Bogota" }).format(new Date());
    const conHoy = await (await fetch(`${baseUrl}/empleados?estado=RETIRADO&desde=${hoy}&hasta=${hoy}`)).json();
    const otroRango = await (
      await fetch(`${baseUrl}/empleados?estado=RETIRADO&desde=2020-01-01&hasta=2020-12-31`)
    ).json();

    assert.deepEqual(conHoy.map((e) => e.id), ["E002"]);
    assert.deepEqual(otroRango, []);
  });

  it("GET con un filtro inválido responde 400", async () => {
    const respuesta = await fetch(`${baseUrl}/empleados?estado=RETIRADO&desde=ayer`);
    assert.equal(respuesta.status, 400);
  });
});

describe("Documentación OpenAPI bajo el prefijo /empleados (alcanzable por el Gateway)", () => {
  let servidor;
  let baseUrl;

  before(async () => {
    const servicio = crearServicioEmpleados(crearRepositorioEmpleadosEnMemoria(), clienteDepartamentosFalso);
    servidor = await new Promise((resolve) => {
      const s = crearApp(servicio).listen(0, () => resolve(s));
    });
    baseUrl = `http://127.0.0.1:${servidor.address().port}`;
  });

  after(() => servidor.close());

  it("sirve Swagger UI en /empleados/docs", async () => {
    const respuesta = await fetch(`${baseUrl}/empleados/docs/`);
    assert.equal(respuesta.status, 200);
    assert.match(await respuesta.text(), /swagger-ui/i);
  });

  it("sirve la especificación en /empleados/openapi.json con los endpoints del Reto 4", async () => {
    const especificacion = await (await fetch(`${baseUrl}/empleados/openapi.json`)).json();
    assert.deepEqual(Object.keys(especificacion.paths["/empleados/{id}"]).sort(), ["delete", "get", "put"]);
  });
});
