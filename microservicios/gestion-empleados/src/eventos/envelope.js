const { randomUUID } = require("node:crypto");

const VERSION_ESQUEMA = 1;

// Instante en UTC, ISO-8601 sin milisegundos, igual que los ejemplos del Catálogo
// de Eventos ("2026-03-01T14:32:05Z").
function instanteUtc(fecha) {
  return fecha.toISOString().replace(/\.\d{3}Z$/, "Z");
}

// Sobre técnico común a todos los eventos (Catálogo de Eventos, sección 2).
// "id" identifica al MENSAJE, no a la entidad: es lo que usan los consumidores para
// descartar duplicados.
function crearEnvelope({ type, data, producer, id = randomUUID(), ahora = new Date() }) {
  return {
    id,
    type,
    version: VERSION_ESQUEMA,
    occurredAt: instanteUtc(ahora),
    producer,
    data
  };
}

module.exports = { crearEnvelope, instanteUtc };
