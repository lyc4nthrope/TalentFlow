const express = require('express');
const { createProxyMiddleware } = require('http-proxy-middleware');

// Middlewares de Seguridad y Autorización
const authMiddleware = require('../middlewares/authMiddleware'); // Retorna 401 si falla el JWT
const authorizeMiddleware = require('../middlewares/authorizeMiddleware'); // Retorna 403 si falla RBAC/Propiedad

const router = express.Router();

/**
 * CONFIGURACIÓN DE TARGETS (MICROSERVICIOS)
 * Mapeados directamente a las variables de entorno de tu docker-compose.yml
 */
const SERVICES = {
    EMPLOYEES_SERVICE: process.env.EMPLEADOS_SERVICE_URL || 'http://empleados-service:8081',
    DEPARTMENTS_SERVICE: process.env.DEPARTAMENTOS_SERVICE_URL || 'http://departamentos-service:8082',
    PROFILES_SERVICE: process.env.PERFILES_SERVICE_URL || 'http://perfiles-service:8083',
    NOTIFICATIONS_SERVICE: process.env.NOTIFICACIONES_SERVICE_URL || 'http://notificaciones-service:8084',
    VACATIONS_SERVICE: process.env.VACACIONES_SERVICE_URL || 'http://vacaciones-service:8085',
    AUTH_SERVICE: process.env.AUTH_SERVICE_URL || 'http://auth-service:8086' // Servicio de Identidad (Reto 5)
};

/**
 * Opciones base para los proxies con inyección de headers HTTP
 */
const proxyOptions = (target) => ({
    target,
    changeOrigin: true,
    onProxyReq: (proxyReq, req) => {
        // Si el usuario fue autenticado por authMiddleware, propagamos su contexto en las cabeceras HTTP
        if (req.user) {
            const userId = req.user.userId || req.user.sub;
            const roles = Array.isArray(req.user.roles) 
                ? req.user.roles.join(',') 
                : (req.user.roles || req.user.rol || '');

            proxyReq.setHeader('x-user-id', userId);
            proxyReq.setHeader('x-user-role', roles);
            if (req.user.email) {
                proxyReq.setHeader('x-user-email', req.user.email);
            }
        }
    }
});

// =============================================================
// 1. RUTAS PÚBLICAS (No requieren JWT)
// =============================================================

// Login, Solicitud de recuperación y Reset de contraseña (Reto 5)
router.use('/auth/login', createProxyMiddleware(proxyOptions(SERVICES.AUTH_SERVICE)));
router.use('/auth/recover-password', createProxyMiddleware(proxyOptions(SERVICES.AUTH_SERVICE)));
router.use('/auth/reset-password', createProxyMiddleware(proxyOptions(SERVICES.AUTH_SERVICE)));


// =============================================================
// 2. APLICACIÓN DE MIDDLEWARES DE SEGURIDAD PROTEGIDOS
// Todo lo que se declare después de este punto exigirá 401 (JWT) y 403 (RBAC)
// =============================================================
router.use(authMiddleware);
router.use(authorizeMiddleware);


// =============================================================
// 3. RUTAS PROTEGIDAS REDIRIGIDAS A MICROSERVICIOS
// =============================================================

// Servicio de Autenticación (Operación autenticada de cambio de clave)
// USER/ADMIN: POST /auth/change-password
router.use('/auth/change-password', createProxyMiddleware(proxyOptions(SERVICES.AUTH_SERVICE)));

// Servicio de Empleados (Puerto 8081 en Node.js)
// USER: GET /empleados | ADMIN: GET, POST, PUT, DELETE /empleados
router.use('/empleados', createProxyMiddleware(proxyOptions(SERVICES.EMPLOYEES_SERVICE)));

// Servicio de Departamentos (Puerto 8082 en PHP)
// USER: GET /departamentos | ADMIN: POST, PUT, DELETE /departamentos
router.use('/departamentos', createProxyMiddleware(proxyOptions(SERVICES.DEPARTMENTS_SERVICE)));

// Servicio de Perfiles (Puerto 8083 en Python/FastAPI)
// USER: GET /perfiles/:id, PUT /perfiles/:empleadoId (propio) | ADMIN: Todos los métodos
router.use('/perfiles', createProxyMiddleware(proxyOptions(SERVICES.PROFILES_SERVICE)));

// Servicio de Vacaciones (Puerto 8085 en Java/Spring Boot)
// USER: GET /vacaciones | ADMIN: POST, PUT, DELETE /vacaciones
router.use('/vacaciones', createProxyMiddleware(proxyOptions(SERVICES.VACATIONS_SERVICE)));

module.exports = router;