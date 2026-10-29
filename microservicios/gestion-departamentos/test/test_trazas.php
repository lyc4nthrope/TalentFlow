<?php

/**
 * Prueba unitaria de src/Trazas.php (Reto 8). Sin dependencias: php test/test_trazas.php
 * (o: docker run --rm -v "$PWD":/app -w /app php:8.2-cli-alpine php test/test_trazas.php).
 */

require __DIR__ . '/../src/Trazas.php';

$fallos = 0;
function comprobar(bool $condicion, string $descripcion): void
{
    global $fallos;
    echo ($condicion ? '  ✔ ' : '  ✘ ') . $descripcion . "\n";
    if (!$condicion) {
        $fallos++;
    }
}

$enviados = [];
$enviar = function (string $url, string $cuerpo) use (&$enviados): void {
    $enviados[] = [$url, json_decode($cuerpo, true)];
};

echo "Propagación W3C\n";
$recibido = '00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01';
$t = new Trazas('http://jaeger:4318', 'departamentos-service', $recibido, $enviar);
comprobar($t->traceId === '4bf92f3577b34da6a3ce929d0e0e4736', 'continúa el traceId recibido');
comprobar($t->padreId === '00f067aa0ba902b7', 'el span del llamador es el padre');
comprobar(preg_match('/^[0-9a-f]{16}$/', $t->spanId) === 1, 'spanId propio de 16 hex');
$sin = new Trazas('http://jaeger:4318', 'x', 'basura', $enviar);
comprobar(preg_match('/^[0-9a-f]{32}$/', $sin->traceId) === 1 && $sin->padreId === null, 'traceparent inválido: traza nueva sin padre');
$ceros = new Trazas('http://jaeger:4318', 'x', '00-' . str_repeat('0', 32) . '-00f067aa0ba902b7-01', $enviar);
comprobar($ceros->padreId === null, 'traceId de ceros (inválido en W3C): traza nueva');

echo "Envío OTLP/JSON\n";
$t->terminar('GET', '/departamentos/{id}', '/departamentos/D01', 404, 1000.5, 1000.75);
[$url, $carga] = $enviados[0];
$span = $carga['resourceSpans'][0]['scopeSpans'][0]['spans'][0];
comprobar($url === 'http://jaeger:4318/v1/traces', 'a /v1/traces del endpoint');
comprobar($carga['resourceSpans'][0]['resource']['attributes'][0]['value']['stringValue'] === 'departamentos-service', 'service.name');
comprobar($span['name'] === 'GET /departamentos/{id}' && $span['kind'] === 2, 'span SERVER nombrado por patrón');
comprobar($span['parentSpanId'] === '00f067aa0ba902b7', 'parentSpanId en la carga');
comprobar($span['startTimeUnixNano'] === '1000500000000' && $span['endTimeUnixNano'] === '1000750000000', 'tiempos en ns');
comprobar($span['status']['code'] === 0, 'un 404 no es error del servidor');
$t->terminar('POST', '/departamentos', '/departamentos', 500, 1, 2);
comprobar($enviados[1][1]['resourceSpans'][0]['scopeSpans'][0]['spans'][0]['status']['code'] === 2, 'un 500 marca el span con error');

$apagado = new Trazas(null, 'x', null, $enviar);
$apagado->terminar('GET', '/departamentos', '/departamentos', 200, 1, 2);
comprobar(count($enviados) === 2, 'apagado (sin endpoint) no envía nada');

echo $fallos === 0 ? "OK\n" : "$fallos fallos\n";
exit($fallos === 0 ? 0 : 1);
