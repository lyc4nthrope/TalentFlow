// Punto de entrada: lee la configuración del entorno y abre el puerto. La lógica vive en
// app.js (construible sin red, para las pruebas).
const { crearApp } = require('./app');
const { crearMetricas, crearAppMetricas } = require('./metricas');

const PORT = process.env.PORT || 8080;
// Reto 5, D11/D21: el secreto es obligatorio y no tiene valor por defecto en el código.
// Lo inyecta Compose (el mismo que recibe auth-service); sin él, el Gateway no arranca.
const JWT_SECRET = process.env.JWT_SECRET;
// Un valor de solo espacios cuenta como ausente: firmar con él sería trivialmente adivinable.
if (!JWT_SECRET || JWT_SECRET.trim() === '') {
    console.error('API Gateway: falta la variable de entorno JWT_SECRET; no se puede validar ningún token. No se inicia.');
    process.exit(1);
}
// El emisor no es un secreto: tiene valor por defecto.
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

const metricas = crearMetricas();
crearApp({ jwtSecret: JWT_SECRET, jwtIssuer: JWT_ISSUER, servicios: SERVICIOS, metricas }).listen(PORT, () => {
    console.log(`API Gateway ejecutandose en el puerto ${PORT}`);
});

// Reto 8, D13: /metrics en un puerto interno aparte (solo expose en Compose), nunca en el
// puerto público: Prometheus lo lee por la red de Docker y el cliente no puede verlo.
const METRICS_PORT = process.env.METRICS_PORT || 9464;
crearAppMetricas(metricas.registro).listen(METRICS_PORT, () => {
    console.log(`Métricas del API Gateway en el puerto interno ${METRICS_PORT}`);
});
