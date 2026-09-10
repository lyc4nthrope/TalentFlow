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
const clienteDepartamentosFalso = { existe: async () => true };

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
      const servicioSinDepto = crearServicioEmpleados(repositorio, { existe: async () => false });
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
        method: "DELETE"
      });

      assert.equal(respuesta.status, 404);
      const cuerpo = await respuesta.json();
      assert.equal(cuerpo.message, "Recurso no encontrado");
    });
  });
});