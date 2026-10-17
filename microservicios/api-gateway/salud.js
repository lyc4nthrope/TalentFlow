// Salud agregada del sistema, vista desde la única URL base (el Gateway).
// Funciones puras + consulta con fetch inyectable: se prueban sin red (tests/salud.test.js).

// Mayor que el tiempo que cada servicio se da para revisar su propia BD (≤ 2 s): si
// fueran iguales, un servicio con la BD caída se reportaría como "No responde" en vez de
// explicar que su BD no responde.
const TIMEOUT_HEALTH_MS = 3000;

// Consulta el /health interno de un servicio. Nunca lanza: si no responde a tiempo, o
// responde algo que no es JSON, lo reporta como DOWN con el motivo.
async function consultarSalud(url, { fetchImpl = fetch, timeoutMs = TIMEOUT_HEALTH_MS } = {}) {
    try {
        const respuesta = await fetchImpl(`${url}/health`, { signal: AbortSignal.timeout(timeoutMs) });
        const cuerpo = await respuesta.json();
        return { status: cuerpo.status ?? 'DOWN', ...cuerpo.components };
    } catch {
        return { status: 'DOWN', detalle: 'No responde' };
    }
}

// Explica POR QUÉ el sistema no está OK (servicio caído, circuito abierto, servicio sin
// conexión al broker). El estado del broker se deduce de lo que reporta cada servicio: el
// Gateway no necesita credenciales de RabbitMQ para saberlo.
function detectarProblemas(servicios) {
    const problemas = [];
    for (const [nombre, estado] of Object.entries(servicios)) {
        if (estado.status !== 'UP') {
            const motivo = estado.detalle ?? (estado.db === 'DOWN' ? 'su base de datos no responde' : 'DOWN');
            problemas.push(`${nombre}: ${motivo}`);
        }
        if (estado.circuitoDepartamentos !== undefined && estado.circuitoDepartamentos !== 'CLOSED') {
            problemas.push(`${nombre}: Circuit Breaker hacia departamentos ${estado.circuitoDepartamentos}`);
        }
        if (estado.broker !== undefined && estado.broker !== 'UP') {
            problemas.push(`${nombre}: sin conexión al broker de mensajería`);
        }
    }
    return problemas;
}

// Salud propia del Gateway (status UP mientras esté vivo; por eso sirve para su
// healthcheck de Docker) + el estado de cada servicio. "sistema" es OK solo si no hay
// ningún problema; si no, DEGRADADO y "problemas" dice qué pasa.
async function saludDelSistema(servicios, opciones) {
    const estados = await Promise.all(servicios.map(({ url }) => consultarSalud(url, opciones)));
    const porServicio = Object.fromEntries(servicios.map(({ nombre }, i) => [nombre, estados[i]]));
    const problemas = detectarProblemas(porServicio);
    return {
        status: 'UP',
        service: 'api-gateway',
        timestamp: new Date().toISOString(),
        sistema: problemas.length === 0 ? 'OK' : 'DEGRADADO',
        problemas,
        servicios: porServicio
    };
}

module.exports = { consultarSalud, detectarProblemas, saludDelSistema };
