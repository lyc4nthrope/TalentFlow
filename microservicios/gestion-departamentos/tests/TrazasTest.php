<?php

use PHPUnit\Framework\TestCase;

final class TrazasTest extends TestCase
{
    private const RECIBIDO = '00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01';

    /** @var list<array{string, array}> */
    private array $enviados = [];

    private function trazas(?string $endpoint, ?string $traceparent): Trazas
    {
        return new Trazas($endpoint, 'departamentos-service', $traceparent, function (string $url, string $cuerpo): void {
            $this->enviados[] = [$url, json_decode($cuerpo, true)];
        });
    }

    public function testContinuaLaTrazaRecibida(): void
    {
        $t = $this->trazas('http://jaeger:4318', self::RECIBIDO);

        $this->assertSame('4bf92f3577b34da6a3ce929d0e0e4736', $t->traceId);
        $this->assertSame('00f067aa0ba902b7', $t->padreId);
        $this->assertMatchesRegularExpression('/^[0-9a-f]{16}$/', $t->spanId);
    }

    public function testTraceparentInvalidoIniciaTrazaNuevaSinPadre(): void
    {
        $t = $this->trazas('http://jaeger:4318', 'basura');

        $this->assertMatchesRegularExpression('/^[0-9a-f]{32}$/', $t->traceId);
        $this->assertNull($t->padreId);
    }

    public function testTraceIdDeCerosEsInvalido(): void
    {
        $t = $this->trazas('http://jaeger:4318', '00-' . str_repeat('0', 32) . '-00f067aa0ba902b7-01');

        $this->assertNull($t->padreId);
        $this->assertNotSame(str_repeat('0', 32), $t->traceId);
    }

    public function testEnviaElSpanEnOtlpJson(): void
    {
        $t = $this->trazas('http://jaeger:4318/', self::RECIBIDO);

        $t->terminar('PUT', '/departamentos/{id}', '/departamentos/D01', 404, 1000.5, 1000.75);

        [$url, $carga] = $this->enviados[0];
        $span = $carga['resourceSpans'][0]['scopeSpans'][0]['spans'][0];
        $this->assertSame('http://jaeger:4318/v1/traces', $url);
        $this->assertSame('departamentos-service', $carga['resourceSpans'][0]['resource']['attributes'][0]['value']['stringValue']);
        $this->assertSame('PUT /departamentos/{id}', $span['name']);
        $this->assertSame(2, $span['kind']);
        $this->assertSame('00f067aa0ba902b7', $span['parentSpanId']);
        $this->assertSame('1000500000000', $span['startTimeUnixNano']);
        $this->assertSame('1000750000000', $span['endTimeUnixNano']);
        $this->assertSame(0, $span['status']['code'], 'un 404 no es error del servidor');
        $this->assertContains(['key' => 'url.path', 'value' => ['stringValue' => '/departamentos/D01']], $span['attributes']);
        $this->assertContains(['key' => 'http.response.status_code', 'value' => ['intValue' => '404']], $span['attributes']);
    }

    public function testUn500MarcaElSpanConErrorYSinPadreNoLlevaParent(): void
    {
        $this->trazas('http://jaeger:4318', null)->terminar('POST', '/departamentos', '/departamentos', 500, 1, 2);

        $span = $this->enviados[0][1]['resourceSpans'][0]['scopeSpans'][0]['spans'][0];
        $this->assertSame(2, $span['status']['code']);
        $this->assertArrayNotHasKey('parentSpanId', $span);
    }

    public function testApagadoNoEnviaNada(): void
    {
        $this->trazas(null, null)->terminar('GET', '/departamentos', '/departamentos', 200, 1, 2);

        $this->assertSame([], $this->enviados);
    }

    public function testLineaDeLogJson(): void
    {
        $t = $this->trazas('http://jaeger:4318', self::RECIBIDO);

        $log = json_decode($t->lineaLog('ERROR', 'GET /departamentos/D01 500', ['status' => 500]), true);

        $this->assertSame('ERROR', $log['level']);
        $this->assertSame('departamentos-service', $log['service']);
        $this->assertSame('GET /departamentos/D01 500', $log['message']);
        $this->assertSame('4bf92f3577b34da6a3ce929d0e0e4736', $log['traceId']);
        $this->assertSame($t->spanId, $log['spanId']);
        $this->assertSame(500, $log['status']);
        $this->assertNotFalse(strtotime($log['timestamp']));
    }

    public function testDesdeEntorno(): void
    {
        putenv('OTEL_SDK_DISABLED=true');
        putenv('OTEL_SERVICE_NAME=otro-servicio');
        try {
            $t = Trazas::desdeEntorno(self::RECIBIDO);
        } finally {
            putenv('OTEL_SDK_DISABLED');
            putenv('OTEL_SERVICE_NAME');
        }

        $this->assertSame('4bf92f3577b34da6a3ce929d0e0e4736', $t->traceId);
        $this->assertSame('otro-servicio', json_decode($t->lineaLog('INFO', 'x'), true)['service']);
        // Apagado: terminar() no intenta enviar (no hay endpoint).
        $t->terminar('GET', '/departamentos', '/departamentos', 200, 1, 2);
    }
}
