// Punto de entrada: lee la configuración del entorno y abre el puerto. La lógica vive en
// app.js (construible sin red, para las pruebas).
const { crearApp } = require('./app');

const PORT = process.env.PORT || 8080;
// TODO(F6): el secreto pasa a ser obligatorio (sin valor por defecto en el código).
const JWT_SECRET = process.env.JWT_SECRET || 'talentflow_super_secret_2026';
const JWT_ISSUER = process.env.JWT_ISSUER || 'auth-service';

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

crearApp({ jwtSecret: JWT_SECRET, jwtIssuer: JWT_ISSUER, servicios: SERVICIOS }).listen(PORT, () => {
    console.log(`API Gateway ejecutandose en el puerto ${PORT}`);
});
