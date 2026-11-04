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
        servicios: porServicio,
        // Formato del PDF (Reto 8, D4): cada servicio de atrás, UP o DOWN. El Gateway sigue
        // UP (y 200) aunque alguno caiga: su propia salud no depende de ellos.
        checks: Object.fromEntries(
            Object.entries(porServicio).map(([nombre, estado]) => [nombre, estado.status === 'UP' ? 'UP' : 'DOWN'])
        )
    };
}

const esObjeto = (valor) => typeof valor === 'object' && valor !== null;

// Reto 8, D19: el /health de un servicio tal cual lo responde (código y cuerpo), para verlo
// desde fuera por el Gateway. Nunca lanza: si no responde a tiempo, o no responde JSON, es
// 503 DOWN con el motivo.
async function leerSaludDe({ nombre, url }, { fetchImpl = fetch, timeoutMs = TIMEOUT_HEALTH_MS } = {}) {
    try {
        const respuesta = await fetchImpl(`${url}/health`, { signal: AbortSignal.timeout(timeoutMs) });
        const cuerpo = await respuesta.json();
        // null o un número también son JSON válido: sin esto, leer cuerpo.status lanzaría
        // fuera de aquí y un solo servicio raro tumbaría /health/servicios y el scrape.
        if (!esObjeto(cuerpo)) throw new Error('El /health no devolvió un objeto JSON');
        return { codigo: respuesta.status, cuerpo };
    } catch {
        return { codigo: 503, cuerpo: { status: 'DOWN', service: nombre, detalle: 'No responde' } };
    }
}

const estaSano = ({ codigo, cuerpo }) => codigo === 200 && cuerpo.status === 'UP';

// Reto 8, D18: la salud de todos los servicios en una sola respuesta, con el /health de cada
// uno tal cual. 200 solo si todos están UP; si no, 503. A diferencia de GET /health (que
// sigue en 200 para el healthcheck de Docker), esta URL sí dice si el sistema está sano.
async function saludDeTodos(servicios, opciones) {
    const lecturas = await Promise.all(servicios.map((s) => leerSaludDe(s, opciones)));
    const todosSanos = lecturas.every(estaSano);
    return {
        codigo: todosSanos ? 200 : 503,
        cuerpo: {
            status: todosSanos ? 'UP' : 'DOWN',
            service: 'api-gateway',
            timestamp: new Date().toISOString(),
            checks: Object.fromEntries(servicios.map(({ nombre }, i) => [nombre, estaSano(lecturas[i]) ? 'UP' : 'DOWN'])),
            servicios: Object.fromEntries(servicios.map(({ nombre }, i) => [nombre, lecturas[i].cuerpo]))
        }
    };
}

// Reto 8, D20: la salud como valores de una métrica (1 = UP, 0 = DOWN). Por servicio, su
// estado general (check "status") y cada dependencia de su `checks` (database,
// messageBroker). Un servicio que no responde no trae `checks`: solo su status en 0.
function valoresDeSalud(cuerpos) {
    const valores = [];
    for (const [service, cuerpo] of Object.entries(cuerpos)) {
        valores.push({ labels: { service, check: 'status' }, value: cuerpo.status === 'UP' ? 1 : 0 });
        for (const [check, estado] of Object.entries(esObjeto(cuerpo.checks) ? cuerpo.checks : {})) {
            valores.push({ labels: { service, check }, value: estado === 'UP' ? 1 : 0 });
        }
    }
    return valores;
}

module.exports = { consultarSalud, detectarProblemas, saludDelSistema, leerSaludDe, saludDeTodos, valoresDeSalud };
