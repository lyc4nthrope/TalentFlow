const { describe, it } = require("node:test");
const assert = require("node:assert/strict");
const { EventEmitter } = require("node:events");

const { crearPublicadorEventos } = require("../src/eventos/publicador");

const LOGGER_SILENCIOSO = { info() {}, warn() {}, error() {} };

// Doble de prueba de amqplib: una conexión con un canal de confirmaciones.
function crearBrokerFalso({ fallaConexion = false, exchangeExiste = true, confirma = true } = {}) {
  const estado = { conexiones: 0, conexionesCerradas: 0, publicados: [] };

  async function conectar() {
    estado.conexiones += 1;
    if (fallaConexion) throw new Error("ECONNREFUSED");

    const conexion = new EventEmitter();
    conexion.close = async () => {
      estado.conexionesCerradas += 1;
    };
    conexion.createConfirmChannel = async () => {
      const canal = new EventEmitter();
      canal.checkExchange = async (nombre) => {
        if (!exchangeExiste) throw new Error(`NOT_FOUND - no exchange '${nombre}'`);
      };
      canal.publish = (exchange, routingKey, contenido, opciones, callback) => {
        estado.publicados.push({ exchange, routingKey, opciones, evento: JSON.parse(contenido) });
        if (confirma) callback(null);
        // Si no confirma, el callback nunca se llama: simula un broker colgado.
      };
      return canal;
    };
    estado.ultimaConexion = conexion;
    return conexion;
  }

  return { conectar, estado };
}

function crearPublicador(broker, opciones = {}) {
  return crearPublicadorEventos({
    url: "amqp://prueba",
    exchange: "talentflow.eventos",
    producer: "empleados-service",
    conectar: broker.conectar,
    logger: LOGGER_SILENCIOSO,
    ...opciones
  });
}

describe("Publicador de eventos", () => {
  it("publica el envelope en el exchange con routing key = tipo y lo marca persistente", async () => {
    const broker = crearBrokerFalso();
    const publicador = crearPublicador(broker);

    const ok = await publicador.publicar("empleado.creado", { empleadoId: "E001" });

    assert.equal(ok, true);
    assert.equal(broker.estado.publicados.length, 1);
    const [{ exchange, routingKey, opciones, evento }] = broker.estado.publicados;
    assert.equal(exchange, "talentflow.eventos");
    assert.equal(routingKey, "empleado.creado");
    assert.equal(opciones.persistent, true);
    assert.equal(opciones.contentType, "application/json");
    assert.equal(opciones.messageId, evento.id);
    assert.equal(evento.type, "empleado.creado");
    assert.equal(evento.producer, "empleados-service");
    assert.deepEqual(evento.data, { empleadoId: "E001" });
    assert.equal(publicador.estadoActual(), "UP");
  });

  it("reutiliza la misma conexión entre publicaciones", async () => {
    const broker = crearBrokerFalso();
    const publicador = crearPublicador(broker);

    await publicador.publicar("empleado.creado", {});
    await publicador.publicar("empleado.retirado", {});

    assert.equal(broker.estado.conexiones, 1);
    assert.equal(broker.estado.publicados.length, 2);
  });

  it("no lanza si el broker está caído: devuelve false", async () => {
    const publicador = crearPublicador(crearBrokerFalso({ fallaConexion: true }));

    const ok = await publicador.publicar("empleado.creado", {});

    assert.equal(ok, false);
    assert.equal(publicador.estadoActual(), "DOWN");
  });

  it("reconecta en la siguiente publicación si la conexión se cerró", async () => {
    const broker = crearBrokerFalso();
    const publicador = crearPublicador(broker);

    await publicador.publicar("empleado.creado", {});
    broker.estado.ultimaConexion.emit("close");
    assert.equal(publicador.estadoActual(), "DOWN");

    const ok = await publicador.publicar("empleado.actualizado", {});

    assert.equal(ok, true);
    assert.equal(broker.estado.conexiones, 2);
  });

  it("devuelve false si el broker no confirma dentro del plazo", async () => {
    const publicador = crearPublicador(crearBrokerFalso({ confirma: false }), { timeoutMs: 50 });

    const ok = await publicador.publicar("empleado.creado", {});

    assert.equal(ok, false);
  });

  it("si el exchange no existe, falla y cierra la conexión (sin fugas)", async () => {
    const broker = crearBrokerFalso({ exchangeExiste: false });
    const publicador = crearPublicador(broker);

    const ok = await publicador.publicar("empleado.creado", {});

    assert.equal(ok, false);
    assert.equal(broker.estado.conexionesCerradas, 1);
    assert.equal(broker.estado.publicados.length, 0);
  });
});
