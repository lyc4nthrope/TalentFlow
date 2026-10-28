const express = require('express');
const { createProxyMiddleware } = require('http-proxy-middleware');
const { saludDelSistema } = require('./salud');
const { verificarToken, leerBearer } = require('./autenticacion');
const { analizarRuta, esRutaPublica, decidirAcceso, servicioDeRuta } = require('./autorizacion');
const { middlewareMetricas } = require('./metricas');

// Cabeceras de identidad que el Gateway propaga a los servicios (Reto 5, D10). Nunca se
// aceptan del cliente: se borran en toda petición y solo se agregan tras validar el token.
const CABECERAS_IDENTIDAD = ['x-user-id', 'x-user-email', 'x-user-role'];

const errorJson = (res, status, error, message, req) =>
    res.status(status).json({ error, message, path: req.originalUrl });

/**
 * Construye el Gateway sin abrir el puerto, para poder probarlo.
 *
 * @param {object} config
 * @param {string} config.jwtSecret  clave HS256 compartida con el auth-service
 * @param {string} config.jwtIssuer  emisor esperado del token (claim iss)
 * @param {Array<{nombre: string, prefijo: string, url: string}>} config.servicios
 * @param {Function} [config.salud]  agregador del /health (por defecto, saludDelSistema)
 * @param {object} [config.metricas]  contadores de crearMetricas(); sin ellos no se mide
 */
function crearApp({ jwtSecret, jwtIssuer, servicios, salud = saludDelSistema, metricas }) {
    const app = express();
    const prefijos = servicios.map((s) => s.prefijo);

    // Reto 8, D5: se mide toda petición, incluidas las rechazadas (400/401/403/404).
    if (metricas) app.use(middlewareMetricas(metricas, prefijos));

    // Salud propia del Gateway + estado de cada servicio, su BD, el Circuit Breaker y la
    // conexión de cada uno con el broker; "problemas" explica por qué el sistema está
    // DEGRADADO. Responde 200 mientras el Gateway esté vivo (healthcheck de Docker).
    app.get('/health', async (req, res, next) => {
        try {
            res.status(200).json(await salud(servicios));
        } catch (error) {
            next(error); // Express 4 no captura el rechazo de un handler async por sí solo
        }
    });

    // 1. Ninguna petición llega a un servicio con cabeceras de identidad puestas por el cliente.
    app.use((req, res, next) => {
        for (const cabecera of CABECERAS_IDENTIDAD) delete req.headers[cabecera];
        next();
    });

    // 2. La decisión se toma sobre la ruta tal como se va a enrutar: los trucos de
    //    codificación (%2F, %2E%2E, //, \) que un servicio podría interpretar distinto se
    //    rechazan antes de autorizar. Una ruta que no pertenece a ningún servicio es 404,
    //    con o sin token (comportamiento del Reto 3).
    app.use((req, res, next) => {
        const ruta = analizarRuta(req.path);
        if (!ruta.valida) {
            return errorJson(res, 400, 'Bad Request', 'Ruta inválida', req);
        }
        if (!servicioDeRuta(ruta.segmentos, prefijos)) {
            return errorJson(res, 404, 'Not Found', 'Ruta no enrutada por el API Gateway', req);
        }
        req.ruta = ruta;
        next();
    });

    // 3. Autenticación y autorización (D8, D9). En una ruta pública el token se ignora.
    app.use((req, res, next) => {
        const metodo = req.method.toUpperCase();
        if (esRutaPublica(metodo, req.ruta.segmentos, prefijos)) {
            return next();
        }

        const token = leerBearer(req.headers.authorization);
        if (!token) {
            return errorJson(res, 401, 'UNAUTHORIZED', 'Token requerido para acceder a este recurso.', req);
        }

        let usuario;
        try {
            usuario = verificarToken(token, { secreto: jwtSecret, emisor: jwtIssuer });
        } catch {
            return errorJson(res, 401, 'UNAUTHORIZED', 'Token inválido o expirado.', req);
        }

        if (!decidirAcceso(usuario, metodo, req.ruta.segmentos)) {
            return errorJson(res, 403, 'FORBIDDEN', 'No tiene permisos para realizar esta acción.', req);
        }

        req.headers['x-user-id'] = usuario.sub;
        req.headers['x-user-role'] = usuario.role;
        req.headers['x-user-email'] = usuario.email || '';
        next();
    });

    const manejarErrorProxy = (err, req, res) => {
        console.error(`[Gateway Error] ${err.message}`);
        // Si el servicio ya había empezado a responder, no se puede cambiar el estado: se
        // corta la conexión con el cliente (escribir el 503 lanzaría "headers already sent").
        if (res.headersSent) {
            res.destroy();
            return;
        }
        res.status(503).json({
            error: 'Service Unavailable',
            message: 'El servicio solicitado no está disponible temporalmente',
            path: req.originalUrl
        });
    };

    // Si el servicio corta su respuesta a mitad del cuerpo, http-proxy no avisa por "error":
    // el cliente quedaría esperando el resto para siempre. Se corta también su conexión.
    const cortarSiQuedaIncompleta = (proxyRes, req, res) => {
        proxyRes.on('close', () => {
            if (!proxyRes.complete && !res.writableEnded) {
                console.error(`[Gateway Error] respuesta incompleta del servicio: ${req.originalUrl}`);
                res.destroy();
            }
        });
    };

    // Nota: se monta en '/' (sin prefijo) y se usa pathFilter en vez de mount-path,
    // porque Express recorta el prefijo del mount antes de invocar el proxy y
    // pathRewrite solo puede reponer un string fijo, rompiendo rutas con parámetros
    // como /empleados/:id (verificado: producía /empleadosE001 en vez de /empleados/E001).
    // El filtro respeta los límites de segmento: /empleadosX no es /empleados.
    for (const { prefijo, url } of servicios) {
        app.use(
            createProxyMiddleware({
                target: url,
                changeOrigin: true,
                pathFilter: (ruta) => ruta === prefijo || ruta.startsWith(`${prefijo}/`),
                on: { error: manejarErrorProxy, proxyRes: cortarSiQuedaIncompleta }
            })
        );
    }

    // Respaldo: cualquier ruta que no tomó un proxy responde 404 en JSON.
    app.use((req, res) => errorJson(res, 404, 'Not Found', 'Ruta no enrutada por el API Gateway', req));

    // Error inesperado: 500 en JSON sin detalle interno (el detalle queda solo en el log).
    // eslint-disable-next-line no-unused-vars
    app.use((err, req, res, next) => {
        console.error(`[Gateway Error] ${err.message}`);
        if (res.headersSent) {
            res.destroy();
            return;
        }
        errorJson(res, 500, 'Internal Server Error', 'Error interno del API Gateway', req);
    });

    return app;
}

module.exports = { crearApp, CABECERAS_IDENTIDAD };
