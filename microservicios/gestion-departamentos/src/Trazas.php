<?php

/**
 * Trazas OpenTelemetry mínimas (Reto 8, D1 y D16). El servicio no usa Composer, así que no
 * hay SDK: se continúa la traza del traceparent recibido (W3C) y, al terminar la petición,
 * se envía un span SERVER a Jaeger en OTLP/HTTP con JSON, el mismo protocolo de los otros
 * SDK. OTEL_SDK_DISABLED=true lo apaga. Infraestructura (D7): prueba en test/test_trazas.php.
 */
final class Trazas
{
    private const TRACEPARENT = '/^00-([0-9a-f]{32})-([0-9a-f]{16})-[0-9a-f]{2}$/';
    private const SPAN_SERVIDOR = 2;
    private const ESTADO_ERROR = 2;

    public readonly string $traceId;
    public readonly ?string $padreId;
    public readonly string $spanId;

    /** @param callable(string, string): void $enviar recibe la URL y el cuerpo JSON */
    public function __construct(
        private ?string $endpoint,
        private string $servicio,
        ?string $traceparent,
        private $enviar = null,
    ) {
        $valido = $traceparent !== null && preg_match(self::TRACEPARENT, $traceparent, $m)
            && $m[1] !== str_repeat('0', 32);
        $this->traceId = $valido ? $m[1] : bin2hex(random_bytes(16));
        $this->padreId = $valido ? $m[2] : null;
        $this->spanId = bin2hex(random_bytes(8));
        $this->enviar ??= self::enviarPorHttp(...);
    }

    public static function desdeEntorno(?string $traceparent): self
    {
        $apagado = strtolower((string) getenv('OTEL_SDK_DISABLED')) === 'true';
        return new self(
            $apagado ? null : (getenv('OTEL_EXPORTER_OTLP_ENDPOINT') ?: null),
            getenv('OTEL_SERVICE_NAME') ?: 'departamentos-service',
            $traceparent,
        );
    }

    /** Envía el span de la petición; con el exportador apagado no hace nada. */
    public function terminar(string $metodo, string $ruta, string $uri, int $estado, float $inicio, float $fin): void
    {
        if ($this->endpoint === null) {
            return;
        }
        ($this->enviar)(
            rtrim($this->endpoint, '/') . '/v1/traces',
            json_encode($this->carga($metodo, $ruta, $uri, $estado, $inicio, $fin)),
        );
    }

    /**
     * Log JSON con el formato común de los 7 servicios (Reto 8, F8) y el traceId de esta
     * petición, para que Grafana lo enlace con su traza.
     */
    public function lineaLog(string $nivel, string $mensaje, array $extra = []): string
    {
        return json_encode([
            'timestamp' => (new DateTimeImmutable())->format('Y-m-d\TH:i:s.vp'),
            'level' => $nivel,
            'service' => $this->servicio,
            'message' => $mensaje,
            'traceId' => $this->traceId,
            'spanId' => $this->spanId,
        ] + $extra, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES);
    }

    /** Cuerpo OTLP/JSON (ids en hexadecimal, tiempos en nanosegundos como texto). */
    public function carga(string $metodo, string $ruta, string $uri, int $estado, float $inicio, float $fin): array
    {
        $span = [
            'traceId' => $this->traceId,
            'spanId' => $this->spanId,
            // El patrón y no la URL, como en el resto de servicios: GET /departamentos/{id}.
            'name' => "$metodo $ruta",
            'kind' => self::SPAN_SERVIDOR,
            'startTimeUnixNano' => sprintf('%.0f', $inicio * 1e9),
            'endTimeUnixNano' => sprintf('%.0f', $fin * 1e9),
            'attributes' => [
                self::atributo('http.request.method', $metodo),
                self::atributo('http.route', $ruta),
                self::atributo('url.path', $uri),
                ['key' => 'http.response.status_code', 'value' => ['intValue' => (string) $estado]],
            ],
            'status' => ['code' => $estado >= 500 ? self::ESTADO_ERROR : 0],
        ];
        if ($this->padreId !== null) {
            $span['parentSpanId'] = $this->padreId;
        }
        return ['resourceSpans' => [[
            'resource' => ['attributes' => [self::atributo('service.name', $this->servicio)]],
            'scopeSpans' => [['scope' => ['name' => 'talentflow.departamentos'], 'spans' => [$span]]],
        ]]];
    }

    private static function atributo(string $clave, string $valor): array
    {
        return ['key' => $clave, 'value' => ['stringValue' => $valor]];
    }

    /** Plazo corto y sin errores visibles: una traza perdida nunca rompe la petición. */
    private static function enviarPorHttp(string $url, string $cuerpo): void
    {
        @file_get_contents($url, false, stream_context_create(['http' => [
            'method' => 'POST',
            'header' => "Content-Type: application/json\r\n",
            'content' => $cuerpo,
            'timeout' => 0.5,
            'ignore_errors' => true,
        ]]));
    }
}
