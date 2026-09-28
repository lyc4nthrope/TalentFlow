const express = require('express')
const { createProxyMiddleware } = require('http-proxy-middleware');

const app = express();
const PORT = process.env.PORT || 8080;

const EMPLEADOS_URL = process.env.EMPLEADOS_SERVICE_URL || 'http://empleados-service:8081';
const DEPARTAMENTOS_URL = process.env.DEPARTAMENTOS_SERVICE_URL || 'http://departamentos-service:8082';
const NOTIFICACIONES_URL = process.env.NOTIFICACIONES_SERVICE_URL || 'http://notificaciones-service:8084';
const PERFILES_URL = process.env.PERFILES_SERVICE_URL || 'http://perfiles-service:8083';

const TIMEOUT_HEALTH_MS = 2000;

// Consulta el /health interno de un servicio. Nunca lanza: si no responde a tiempo,
// o responde algo que no es JSON, lo reporta como DOWN con el motivo.
async function consultarSalud(url) {
    try {
        const respuesta = await fetch(`${url}/health`, { signal: AbortSignal.timeout(TIMEOUT_HEALTH_MS) });
        const cuerpo = await respuesta.json();
        return { status: cuerpo.status ?? 'DOWN', ...cuerpo.components };
    } catch (error) {
        return { status: 'DOWN', detalle: 'No responde' };
    }
}

// Salud propia del Gateway (siempre 200 mientras el Gateway esté vivo, por eso sirve
// para su healthcheck de Docker) + el estado de cada servicio y del Circuit Breaker,
// para ver todo el sistema desde la única URL base. "sistema" es OK solo si todo está
// UP y el circuito CLOSED; si no, DEGRADADO.
app.get('/health', async (req, res) => {
    const [empleados, departamentos] = await Promise.all([
        consultarSalud(EMPLEADOS_URL),
        consultarSalud(DEPARTAMENTOS_URL)
    ]);

    const todoOk =
        empleados.status === 'UP' &&
        departamentos.status === 'UP' &&
        empleados.circuitoDepartamentos === 'CLOSED';

    res.status(200).json({
        status: 'UP',
        service: 'api-gateway',
        timestamp: new Date().toISOString(),
        sistema: todoOk ? 'OK' : 'DEGRADADO',
        servicios: {
            'empleados-service': empleados,
            'departamentos-service': departamentos
        }
    });
});

const handleProxyError = (err, req, res) => {
    console.error(`[Gateway Error] ${err.message}`);
    res.status(503).json({
        error: 'Service Unavailable',
        message: 'El servicio solicitado no está disponible temporalmente',
        path: req.originalUrl
    });
};

// Nota: se monta en '/' (sin prefijo) y se usa pathFilter en vez de mount-path,
// porque Express recorta el prefijo del mount antes de invocar el proxy y
// pathRewrite solo puede reponer un string fijo, rompiendo rutas con parámetros
// como /empleados/:id (verificado: producía /empleadosE001 en vez de /empleados/E001).
app.use(
    createProxyMiddleware({
        target: EMPLEADOS_URL,
        changeOrigin: true,
        pathFilter: ['/empleados'],
        on: { error: handleProxyError }
    })
);

app.use(
    createProxyMiddleware({
        target: DEPARTAMENTOS_URL,
        changeOrigin: true,
        pathFilter: ['/departamentos'],
        on: { error: handleProxyError }
    })
);

// Reto 4: servicios nuevos, también detrás del Gateway (solo usan expose:).
app.use(
    createProxyMiddleware({
        target: NOTIFICACIONES_URL,
        changeOrigin: true,
        pathFilter: ['/notificaciones'],
        on: { error: handleProxyError }
    })
);

app.use(
    createProxyMiddleware({
        target: PERFILES_URL,
        changeOrigin: true,
        pathFilter: ['/perfiles'],
        on: { error: handleProxyError }
    })
);

// Cualquier ruta fuera de /health, /empleados/* y /departamentos/*: 404 en JSON con el
// mismo formato que el 503, en vez de la página HTML por defecto de Express.
app.use((req, res) => {
    res.status(404).json({
        error: 'Not Found',
        message: 'Ruta no enrutada por el API Gateway',
        path: req.originalUrl
    });
});

app.listen(PORT, () => {
    console.log(`API Gateway ejecutandose en el puerto ${PORT}`);
});