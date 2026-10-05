const express = require('express');
const { createProxyMiddleware } = require('http-proxy-middleware');
const { saludDelSistema } = require('./salud');

const app = express();
const PORT = process.env.PORT || 8080;

// Tabla única de servicios detrás del Gateway: de aquí salen tanto las rutas como el
// /health agregado. Agregar un servicio = agregar una fila. Todos usan expose: en Compose
// (solo son alcanzables a través del Gateway).
const SERVICIOS = [
    { nombre: 'empleados-service', prefijo: '/empleados', url: process.env.EMPLEADOS_SERVICE_URL || 'http://empleados-service:8081' },
    { nombre: 'departamentos-service', prefijo: '/departamentos', url: process.env.DEPARTAMENTOS_SERVICE_URL || 'http://departamentos-service:8082' },
    // Reto 4
    { nombre: 'perfiles-service', prefijo: '/perfiles', url: process.env.PERFILES_SERVICE_URL || 'http://perfiles-service:8083' },
    { nombre: 'notificaciones-service', prefijo: '/notificaciones', url: process.env.NOTIFICACIONES_SERVICE_URL || 'http://notificaciones-service:8084' },
    { nombre: 'vacaciones-service', prefijo: '/vacaciones', url: process.env.VACACIONES_SERVICE_URL || 'http://vacaciones-service:8085' }
];

// Salud propia del Gateway + estado de cada servicio, su BD, el Circuit Breaker y la
// conexión de cada uno con el broker; "problemas" explica por qué el sistema está
// DEGRADADO. Responde 200 mientras el Gateway esté vivo (healthcheck de Docker).
app.get('/health', async (req, res) => {
    res.status(200).json(await saludDelSistema(SERVICIOS));
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
for (const { prefijo, url } of SERVICIOS) {
    app.use(
        createProxyMiddleware({
            target: url,
            changeOrigin: true,
            pathFilter: [prefijo],
            on: { error: handleProxyError }
        })
    );
}

// Cualquier ruta no enrutada: 404 en JSON con el mismo formato que el 503, en vez de la
// página HTML por defecto de Express.
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
