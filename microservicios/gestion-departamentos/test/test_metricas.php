<?php

/**
 * Prueba unitaria de src/Metricas.php (Reto 8). Sin dependencias: php test/test_metricas.php
 * (o, sin PHP local: docker run --rm -v "$PWD":/app -w /app php:8.2-cli-alpine php test/test_metricas.php).
 */

require __DIR__ . '/../src/Metricas.php';

$fallos = 0;
function comprobar(bool $condicion, string $descripcion): void
{
    global $fallos;
    echo ($condicion ? '  ✔ ' : '  ✘ ') . $descripcion . "\n";
    if (!$condicion) {
        $fallos++;
    }
}

$archivo = tempnam(sys_get_temp_dir(), 'metricas');
$metricas = new Metricas($archivo);

echo "Rutas\n";
comprobar(Metricas::ruta('GET', '/departamentos/D01') === '/departamentos/{id}', 'el id se agrupa en el patrón');
comprobar(Metricas::ruta('POST', '/departamentos') === '/departamentos', 'POST /departamentos');
comprobar(Metricas::ruta('GET', '/departamentos/docs') === '/departamentos/docs', 'docs no se toma como id');
comprobar(Metricas::ruta('DELETE', '/departamentos/D01') === 'other', 'método sin ruta es other');
comprobar(Metricas::ruta('GET', '/no-existe') === 'other', 'ruta desconocida es other');

echo "Exposición\n";
$metricas->registrar('GET', '/departamentos/{id}', 200, 0.003);
$metricas->registrar('GET', '/departamentos/{id}', 200, 1.5);
$metricas->registrar('GET', 'other', 404, 0.001);
// Otra instancia sobre el mismo archivo = otra petición: los contadores se conservan.
$texto = (new Metricas($archivo))->exposicion();

comprobar(str_contains($texto, 'http_requests_total{method="GET",route="/departamentos/{id}",status="200"} 2'), 'cuenta por serie entre peticiones');
comprobar(str_contains($texto, 'http_requests_total{method="GET",route="other",status="404"} 1'), 'serie de 404');
comprobar(str_contains($texto, 'http_request_duration_seconds_bucket{method="GET",route="/departamentos/{id}",status="200",le="0.005"} 1'), 'bucket de 5 ms');
comprobar(str_contains($texto, 'http_request_duration_seconds_bucket{method="GET",route="/departamentos/{id}",status="200",le="2"} 2'), 'bucket de 2 s acumulado');
comprobar(str_contains($texto, 'http_request_duration_seconds_count{method="GET",route="/departamentos/{id}",status="200"} 2'), 'count del histograma');
comprobar(str_contains($texto, '# TYPE http_request_duration_seconds histogram'), 'tipo histogram');
comprobar(str_contains($texto, 'process_cpu_seconds_total '), 'CPU del proceso');
comprobar(str_contains($texto, 'process_resident_memory_bytes '), 'memoria del proceso');
comprobar((new Metricas('/ruta/que/no/existe/x.json'))->exposicion() !== '', 'sin archivo no revienta');

unlink($archivo);
echo $fallos === 0 ? "OK\n" : "$fallos fallos\n";
exit($fallos === 0 ? 0 : 1);
