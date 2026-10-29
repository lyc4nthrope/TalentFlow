// Reto 8, D5: contrato de métricas HTTP de empleados-service.
const { describe, it } = require("node:test");
const assert = require("node:assert/strict");

const { crearApp } = require("../src/app");
const { crearServicioEmpleados } = require("../src/services/empleados.service");
const { crearRepositorioEmpleadosEnMemoria } = require("../src/repository/empleados.repository.memoria");
const { crearMetricas, manejadorMetricas } = require("../src/metricas");

async function conServidor(app, fn) {
  const server = await new Promise((resolve) => {
    const s = app.listen(0, "127.0.0.1", () => resolve(s));
  });
  try {
    return await fn(`http://127.0.0.1:${server.address().port}`);
  } finally {
    server.close();
  }
}

describe("métricas de empleados-service", () => {
  it("route es el patrón de Express, no la URL real; /metrics no se cuenta a sí mismo", async () => {
    const metricas = crearMetricas();
    const servicio = crearServicioEmpleados(crearRepositorioEmpleadosEnMemoria(), { existe: async () => "EXISTE" });
    const app = crearApp(servicio, undefined, { metricas });

    await conServidor(app, async (base) => {
      assert.equal((await fetch(`${base}/empleados`)).status, 200);
      assert.equal((await fetch(`${base}/empleados/E999`)).status, 404);
      assert.equal((await fetch(`${base}/no-existe`)).status, 404);
      assert.equal((await fetch(`${base}/health`)).status, 200);
      const respuesta = await fetch(`${base}/metrics`);
      assert.equal(respuesta.status, 200);
      assert.match(respuesta.headers.get("content-type"), /^text\/plain/);
    });

    const texto = await metricas.registro.metrics();
    assert.match(texto, /http_requests_total\{method="GET",route="\/empleados",status="200"\} 1/);
    assert.match(texto, /http_requests_total\{method="GET",route="\/empleados\/:id",status="404"\} 1/);
    assert.match(texto, /http_requests_total\{method="GET",route="other",status="404"\} 1/);
    assert.match(texto, /http_requests_total\{method="GET",route="\/health",status="200"\} 1/);
    assert.doesNotMatch(texto, /route="\/metrics"/);
    assert.doesNotMatch(texto, /E999/);
    assert.match(texto, /http_request_duration_seconds_bucket\{le="2",method="GET",route="\/empleados",status="200"\} 1/);
    assert.match(texto, /process_resident_memory_bytes/);
  });

  it("un fallo al serializar responde 500", async () => {
    const registro = { contentType: "text/plain", metrics: async () => { throw new Error("x"); } };
    const res = { status(c) { this.codigo = c; return this; }, end() {}, set() { return this; } };
    const original = console.error;
    console.error = () => {};
    try {
      await manejadorMetricas(registro)({}, res);
    } finally {
      console.error = original;
    }
    assert.equal(res.codigo, 500);
  });
});
