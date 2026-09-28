const amqp = require("amqplib");
const { crearEnvelope } = require("./envelope");

function conTimeout(promesa, ms, descripcion) {
  let timer;
  const plazo = new Promise((_, reject) => {
    timer = setTimeout(() => reject(new Error(`${descripcion}: sin respuesta en ${ms}ms`)), ms);
  });
  return Promise.race([promesa, plazo]).finally(() => clearTimeout(timer));
}

// Publicador de eventos hacia RabbitMQ.
//
// Contrato: publicar() NUNCA lanza. Devuelve true si el broker confirmó el mensaje y
// false si no se pudo publicar (el error queda en el log). Así lo exige el Reto 4: si
// la publicación falla, se registra el error pero no se revierte la operación en BD.
//
// - Usa un canal con confirmaciones (publisher confirms): "publicado" significa que el
//   broker lo recibió y lo aceptó, no solo que salió del proceso.
// - La conexión se crea al arrancar (o al primer uso) y, si se pierde, se reintenta en
//   segundo plano con espera exponencial (y también en la siguiente publicación): si
//   RabbitMQ está caído, empleados-service sigue atendiendo, y /health vuelve a UP en
//   cuanto el broker regresa, sin esperar a que alguien publique.
// - Cada publicación tiene un tiempo máximo, para no bloquear la petición HTTP.
function crearPublicadorEventos({
  url,
  exchange,
  producer,
  timeoutMs = 3000,
  conectar = amqp.connect,
  logger = console,
  esperaReconexionMs = 1000,
  esperaReconexionMaximaMs = 30000
}) {
  let canalPromesa = null;
  let conectado = false;
  let reconexionPendiente = null;
  let espera = esperaReconexionMs;

  function reiniciar() {
    canalPromesa = null;
    conectado = false;
  }

  function programarReconexion() {
    if (reconexionPendiente) return;
    reconexionPendiente = setTimeout(async () => {
      reconexionPendiente = null;
      try {
        await conTimeout(obtenerCanal(), timeoutMs, "Reconexión con el broker");
      } catch {
        espera = Math.min(espera * 2, esperaReconexionMaximaMs);
        programarReconexion();
      }
    }, espera);
    // No mantiene vivo el proceso solo por este temporizador.
    reconexionPendiente.unref?.();
  }

  function obtenerCanal() {
    if (!canalPromesa) {
      canalPromesa = (async () => {
        const conexion = await conectar(url, { timeout: timeoutMs });
        conexion.on("error", (error) => logger.error(`Conexión con el broker: ${error.message}`));
        conexion.on("close", () => {
          logger.warn("Conexión con el broker cerrada; se reintentará en segundo plano");
          reiniciar();
          programarReconexion();
        });
        try {
          const canal = await conexion.createConfirmChannel();
          canal.on("error", (error) => logger.error(`Canal del broker: ${error.message}`));
          canal.on("close", reiniciar);
          // Falla si el exchange no existe (topología no importada): mejor un error
          // claro que publicar a un destino inexistente.
          await canal.checkExchange(exchange);
          conectado = true;
          espera = esperaReconexionMs;
          logger.info(`Conectado al broker (exchange ${exchange})`);
          return canal;
        } catch (error) {
          // Sin esto, cada reintento fallido dejaría una conexión TCP abierta.
          await conexion.close().catch(() => {});
          throw error;
        }
      })().catch((error) => {
        reiniciar();
        throw error;
      });
    }
    return canalPromesa;
  }

  async function publicar(type, data) {
    const evento = crearEnvelope({ type, data, producer });
    try {
      const canal = await conTimeout(obtenerCanal(), timeoutMs, "Conexión con el broker");
      const contenido = Buffer.from(JSON.stringify(evento));
      await conTimeout(
        new Promise((resolve, reject) => {
          canal.publish(
            exchange,
            type, // routing key = tipo de evento: así el exchange topic lo enruta a cada cola
            contenido,
            {
              persistent: true,
              contentType: "application/json",
              messageId: evento.id,
              type,
              appId: producer
            },
            (error) => (error ? reject(error) : resolve())
          );
        }),
        timeoutMs,
        "Confirmación del broker"
      );
      logger.info(`📤 Evento publicado: ${type} (id ${evento.id})`);
      return true;
    } catch (error) {
      logger.error(`❌ No se pudo publicar ${type} (id ${evento.id}): ${error.message}`);
      return false;
    }
  }

  // Intenta conectar al arrancar para detectar problemas temprano; si falla no es
  // fatal: el servicio arranca igual y la conexión se reintenta al publicar.
  async function iniciar() {
    try {
      await conTimeout(obtenerCanal(), timeoutMs, "Conexión con el broker");
    } catch (error) {
      logger.warn(`Broker no disponible al arrancar (${error.message}); se reintentará en segundo plano`);
      programarReconexion();
    }
  }

  function estadoActual() {
    return conectado ? "UP" : "DOWN";
  }

  return { publicar, iniciar, estadoActual };
}

module.exports = { crearPublicadorEventos };
