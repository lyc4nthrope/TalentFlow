// Trazas OpenTelemetry (Reto 8, D1 y D7). Se carga antes que la app (node --require) para
// que las librerías queden instrumentadas al importarse. Configuración por entorno:
// OTEL_SERVICE_NAME y OTEL_EXPORTER_OTLP_ENDPOINT (http://jaeger:4318, OTLP/HTTP);
// OTEL_SDK_DISABLED=true lo apaga. Muestreo: el de por defecto, 100 % (D12).
const { NodeSDK } = require('@opentelemetry/sdk-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-http');
const { HttpInstrumentation } = require('@opentelemetry/instrumentation-http');
const { ExpressInstrumentation } = require('@opentelemetry/instrumentation-express');
const { UndiciInstrumentation } = require('@opentelemetry/instrumentation-undici');
const { PgInstrumentation } = require('@opentelemetry/instrumentation-pg');
const { AmqplibInstrumentation } = require('@opentelemetry/instrumentation-amqplib');

// Healthchecks (cada 5 s) y scrapes de Prometheus: sin esto taparían las trazas de negocio.
const RUTAS_RUIDO = new Set(['/health', '/metrics']);
const esRuido = (ruta = '') => RUTAS_RUIDO.has(ruta.split('?')[0]);

const sdk = new NodeSDK({
    traceExporter: new OTLPTraceExporter(),
    instrumentations: [
        new HttpInstrumentation({
            ignoreIncomingRequestHook: (peticion) => esRuido(peticion.url),
            ignoreOutgoingRequestHook: (opciones) => esRuido(opciones.path)
        }),
        // Sin un span por cada middleware (cors, json, auth...): solo la ruta y su handler.
        new ExpressInstrumentation({ ignoreLayersType: ['middleware'] }),
        // fetch nativo de Node (undici): llamadas a otros servicios, p. ej. a departamentos.
        new UndiciInstrumentation({ ignoreRequestHook: (peticion) => esRuido(peticion.path) }),
        new PgInstrumentation(),
        // RabbitMQ (D10): inyecta traceparent en las cabeceras de cada mensaje publicado.
        new AmqplibInstrumentation()
    ]
});
sdk.start();
