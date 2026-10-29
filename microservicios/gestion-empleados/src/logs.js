// Logs JSON (Reto 8, F8): una línea por mensaje con el formato común de los 7 servicios
// (timestamp, level, service, message, traceId, spanId), que Promtail envía a Loki y Grafana
// enlaza con la traza en Jaeger. Se activa desde tracing.js, así cada console.* existente
// queda en JSON sin tocar el código que loguea. Infraestructura (D7), con su prueba.
const util = require('node:util');
const { trace, isSpanContextValid } = require('@opentelemetry/api');

const NIVELES = { debug: 'DEBUG', log: 'INFO', info: 'INFO', warn: 'WARN', error: 'ERROR' };

function lineaJson(nivel, args, servicio, span = trace.getActiveSpan()) {
    const linea = { timestamp: new Date().toISOString(), level: nivel, service: servicio, message: util.format(...args) };
    const contexto = span?.spanContext();
    if (contexto && isSpanContextValid(contexto)) {
        linea.traceId = contexto.traceId;
        linea.spanId = contexto.spanId;
    }
    return JSON.stringify(linea);
}

function activarLogsJson(servicio, consola = console, proceso = process) {
    for (const [metodo, nivel] of Object.entries(NIVELES)) {
        const salida = nivel === 'WARN' || nivel === 'ERROR' ? proceso.stderr : proceso.stdout;
        consola[metodo] = (...args) => salida.write(`${lineaJson(nivel, args, servicio)}\n`);
    }
    // Los avisos de Node (p. ej. una API obsoleta en una dependencia) son WARN, no ERROR. El
    // proceso arranca con --no-warnings para que Node no los imprima además por su cuenta.
    proceso.on('warning', (aviso) => consola.warn(`${aviso.name}: ${aviso.message}`));
}

module.exports = { lineaJson, activarLogsJson };
