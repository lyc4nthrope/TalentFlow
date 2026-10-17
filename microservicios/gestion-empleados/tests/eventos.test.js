const { describe, it } = require("node:test");
const assert = require("node:assert/strict");

const { crearEnvelope } = require("../src/eventos/envelope");
const {
  TIPOS_EVENTO_EMPLEADO,
  datosEmpleadoCreado,
  datosEmpleadoActualizado,
  datosEmpleadoRetirado
} = require("../src/eventos/eventos-empleado");

const EMPLEADO = {
  id: "E001",
  nombre: "Juan",
  apellido: "Pérez",
  email: "juan.perez@empresa.com",
  numeroEmpleado: "EMP-2026-001",
  cargo: "Desarrollador Senior",
  area: "Tecnología",
  departamentoId: "IT",
  fechaIngreso: "2026-02-10",
  estado: "ACTIVO",
  validacionDepartamento: "ACEPTADO",
  fechaRetiro: null,
  motivoRetiro: null
};

// Estas pruebas fijan el contrato del Catálogo de Eventos: si alguien agrega, quita o
// renombra un campo, fallan. Las claves se comparan exactas (ni una más, ni una menos).
describe("Envelope (Catálogo de Eventos, sección 2)", () => {
  it("tiene exactamente id, type, version, occurredAt, producer y data", () => {
    const evento = crearEnvelope({ type: "empleado.creado", data: {}, producer: "empleados-service" });
    assert.deepEqual(Object.keys(evento).sort(), ["data", "id", "occurredAt", "producer", "type", "version"]);
  });

  it("usa un UUID como id de mensaje, versión 1 y occurredAt en UTC sin milisegundos", () => {
    const evento = crearEnvelope({
      type: "empleado.creado",
      data: {},
      producer: "empleados-service",
      ahora: new Date("2026-03-01T14:32:05.789Z")
    });
    assert.match(evento.id, /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
    assert.equal(evento.version, 1);
    assert.equal(evento.occurredAt, "2026-03-01T14:32:05Z");
    assert.equal(evento.producer, "empleados-service");
  });

  it("genera un id distinto por mensaje", () => {
    const a = crearEnvelope({ type: "x.y", data: {}, producer: "p" });
    const b = crearEnvelope({ type: "x.y", data: {}, producer: "p" });
    assert.notEqual(a.id, b.id);
  });
});

describe("Cargas útiles de los eventos de empleado (Catálogo 3.1 a 3.3)", () => {
  it("los nombres de evento siguen <entidad>.<acción-en-pasado>", () => {
    assert.deepEqual(TIPOS_EVENTO_EMPLEADO, {
      CREADO: "empleado.creado",
      ACTUALIZADO: "empleado.actualizado",
      RETIRADO: "empleado.retirado"
    });
  });

  it("3.1 empleado.creado", () => {
    assert.deepEqual(datosEmpleadoCreado(EMPLEADO), {
      empleadoId: "E001",
      nombre: "Juan",
      apellido: "Pérez",
      email: "juan.perez@empresa.com",
      numeroEmpleado: "EMP-2026-001",
      cargo: "Desarrollador Senior",
      area: "Tecnología",
      departamentoId: "IT",
      fechaIngreso: "2026-02-10",
      estado: "ACTIVO"
    });
  });

  it("3.2 empleado.actualizado", () => {
    assert.deepEqual(datosEmpleadoActualizado(EMPLEADO), {
      empleadoId: "E001",
      nombre: "Juan",
      apellido: "Pérez",
      email: "juan.perez@empresa.com",
      cargo: "Desarrollador Senior",
      area: "Tecnología",
      departamentoId: "IT"
    });
  });

  it("3.3 empleado.retirado", () => {
    const retirado = {
      ...EMPLEADO,
      estado: "RETIRADO",
      fechaRetiro: "2026-11-30T16:45:00Z",
      motivoRetiro: "RENUNCIA"
    };
    assert.deepEqual(datosEmpleadoRetirado(retirado), {
      empleadoId: "E001",
      email: "juan.perez@empresa.com",
      fechaRetiro: "2026-11-30T16:45:00Z",
      motivo: "RENUNCIA"
    });
  });
});
