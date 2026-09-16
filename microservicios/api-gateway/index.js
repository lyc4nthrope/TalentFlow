const express = require('express')
const { createProxyMiddleware } = require('http-proxy-middleware');

const app = express();
const PORT = process.env.PORT || 8080;

const EMPLEADOS_URL = process.env.EMPLEADOS_SERVICE_URL || 'http://empleados-service:8081';
const DEPARTAMENTOS_URL = process.env.DEPARTAMENTOS_SERVICE_URL || 'http://departamentos-service:8082';

// ENDpoint
app.get('/health', (req, res) => {
    res.status(200).json({ status: 'UP', service: 'api-gateway' });
});

const handleProxyError = (err, req, res) => {
    console.error(`[Gateway Error] ${err.message}`);
    res.status(503).json({
        error: 'Service Unavailable',
        message: 'El servicio solicitado no está disponible temporalmente',
        path: req.originalUrl
    });
};

app.use(
    ['/empleados', '/docs', '/openapi.json'],
    createProxyMiddleware({
        target: EMPLEADOS_URL,
        changeOrigin: true,
        pathRewrite: { '^/': '/empleados' }, // Fuerza a mantener la ruta /empleados
        onError: handleProxyError
    })
);

app.use(
    '/departamentos',
    createProxyMiddleware({
        target: DEPARTAMENTOS_URL,
        changeOrigin: true,
        pathRewrite: { '^/': '/departamentos' },
        onError: handleProxyError
    })
);

app.listen(PORT, () => {
    console.log(`API Gateway ejecutandose en el puerto ${PORT}`);
});