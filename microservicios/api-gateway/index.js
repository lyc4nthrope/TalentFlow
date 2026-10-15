const express = require('express');
const jwt = require('jsonwebtoken');
const { createProxyMiddleware } = require('http-proxy-middleware');
const { saludDelSistema } = require('./salud');

const app = express();
const PORT = process.env.PORT || 8080;
const JWT_SECRET = process.env.JWT_SECRET || 'talentflow_super_secret_2026';

// Tabla única de servicios detrás del Gateway: de aquí salen tanto las rutas como el
// /health agregado. Agregar un servicio = agregar una fila. Todos usan expose: en Compose
// (solo son alcanzables a través del Gateway).
const SERVICIOS = [
    { nombre: 'auth-service', prefijo: '/auth', url: process.env.AUTH_SERVICE_URL || 'http://auth-service:8086' },
    { nombre: 'empleados-service', prefijo: '/empleados', url: process.env.EMPLEADOS_SERVICE_URL || 'http://empleados-service:8081' },
    { nombre: 'departamentos-service', prefijo: '/departamentos', url: process.env.DEPARTAMENTOS_SERVICE_URL || 'http://departamentos-service:8082' },
    // Reto 4
    { nombre: 'perfiles-service', prefijo: '/perfiles', url: process.env.PERFILES_SERVICE_URL || 'http://perfiles-service:8083' },
    { nombre: 'notificaciones-service', prefijo: '/notificaciones', url: process.env.NOTIFICACIONES_SERVICE_URL || 'http://notificaciones-service:8084' },
    { nombre: 'vacaciones-service', prefijo: '/vacaciones', url: process.env.VACACIONES_SERVICE_URL || 'http://vacaciones-service:8085' }
];

const PUBLIC_PATHS = [
    '/health',
    '/auth/login',
    '/auth/recover-password',
    '/auth/reset-password'
];

const readToken = (headerValue) => {
    if (!headerValue) {
        return null;
    }

    const match = /^Bearer\s+(.+)$/i.exec(headerValue);
    return match ? match[1] : null;
};

const getUserFromToken = (token) => {
    const payload = jwt.verify(token, JWT_SECRET);

    // Normalizar roles
    let roles = [];
    if (Array.isArray(payload.roles)) {
        roles = payload.roles;
    } else if (payload.role) {
        roles = [payload.role];
    } else if (payload.roles) {
        roles = [payload.roles];
    }

    // El ID del usuario/empleado puede venir en sub, empleadoId o userId
    const userId = payload.empleadoId || payload.sub || payload.userId || payload.id;

    return {
        ...payload,
        userId: userId ? String(userId) : null,
        email: payload.email,
        roles
    };
};

const ensureAuthenticated = (req, res, next) => {
    const path = req.path || req.originalUrl || '/';

    if (PUBLIC_PATHS.includes(path) || PUBLIC_PATHS.some(publicPath => path.startsWith(`${publicPath}/`))) {
        return next();
    }

    const token = readToken(req.headers.authorization);
    if (!token) {
        return res.status(401).json({
            error: 'UNAUTHORIZED',
            message: 'Token requerido para acceder a este recurso.'
        });
    }

    try {
        const user = getUserFromToken(token);
        req.user = user;
        req.headers['x-user-id'] = user.userId;
        req.headers['x-user-email'] = user.email || '';
        req.headers['x-user-role'] = rolesToHeader(user.roles);
        next();
    } catch (error) {
        return res.status(401).json({
            error: 'UNAUTHORIZED',
            message: 'Token inválido o expirado.'
        });
    }
};

const rolesToHeader = (roles = []) => Array.isArray(roles) ? roles.join(',') : String(roles || '');

const checkPermissions = (req, res, next) => {
    const path = (req.path || req.originalUrl || '/').split('?')[0];
    const method = (req.method || 'GET').toUpperCase();

    if (PUBLIC_PATHS.includes(path) || PUBLIC_PATHS.some(publicPath => path.startsWith(`${publicPath}/`))) {
        return next();
    }

    const user = req.user || {};
    const roles = Array.isArray(user.roles) ? user.roles : (user.roles ? [user.roles] : []);

    // 1. Rol ADMIN tiene acceso total
    if (roles.includes('ADMIN')) {
        return next();
    }

    // 2. Cambio de contraseña propio
    if (method === 'POST' && path === '/auth/change-password') {
        return next();
    }

    // 3. Regla para USER: Lectura (GET) permitida en todos los recursos del ecosistema
    if (method === 'GET') {
        return next();
    }

    // 4. Regla para USER: Escritura en Perfiles únicamente si es dueño de su perfil
    if (path.startsWith('/perfiles/')) {
        const targetEmpleadoId = path.split('/perfiles/')[1]?.split('/')[0];

        if ((method === 'PUT' || method === 'PATCH') && user.userId && String(user.userId) === String(targetEmpleadoId)) {
            return next();
        }

        return res.status(403).json({
            error: 'FORBIDDEN',
            message: 'No tiene permisos para modificar este perfil.'
        });
    }

    // Cualquier otra acción de escritura (POST, PUT, DELETE) por un USER es rechazada
    return res.status(403).json({
        error: 'FORBIDDEN',
        message: 'No tiene permisos para realizar esta acción.'
    });
};

// Salud propia del Gateway + estado de cada servicio, su BD, el Circuit Breaker y la
// conexión de cada uno con el broker; "problemas" explica por qué el sistema está
// DEGRADADO. Responde 200 mientras el Gateway esté vivo (healthcheck de Docker).
app.get('/health', async (req, res) => {
    res.status(200).json(await saludDelSistema(SERVICIOS));
});

app.use(ensureAuthenticated);
app.use(checkPermissions);

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


// Test - Iniciar el servidor solo si no estamos en entorno de pruebas (ej. Jest/Supertest)
if (process.env.NODE_ENV !== 'test') {
    const PORT = process.env.PORT || 8080;
    app.listen(PORT, () => {
        console.log(`API Gateway ejecutandose en el puerto ${PORT}`);
    });
}

module.exports = app;