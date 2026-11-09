<?php

use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

final class MetricasTest extends TestCase
{
    private string $archivo;

    protected function setUp(): void
    {
        $this->archivo = tempnam(sys_get_temp_dir(), 'metricas');
    }

    protected function tearDown(): void
    {
        @unlink($this->archivo);
    }

    /** @return array<string, array{string, string, string}> */
    public static function rutas(): array
    {
        return [
            'el id se agrupa en el patrón' => ['GET', '/departamentos/D01', '/departamentos/{id}'],
            'PUT con id' => ['PUT', '/departamentos/D01', '/departamentos/{id}'],
            'DELETE con id' => ['DELETE', '/departamentos/D01', '/departamentos/{id}'],
            'POST /departamentos' => ['POST', '/departamentos', '/departamentos'],
            'GET /departamentos' => ['GET', '/departamentos', '/departamentos'],
            'docs no se toma como id' => ['GET', '/departamentos/docs', '/departamentos/docs'],
            'openapi.json' => ['GET', '/departamentos/openapi.json', '/departamentos/openapi.json'],
            'health' => ['GET', '/health', '/health'],
            'método sin ruta es other' => ['PATCH', '/departamentos/D01', 'other'],
            'DELETE de la colección es other' => ['DELETE', '/departamentos', 'other'],
            'subruta es other' => ['GET', '/departamentos/D01/x', 'other'],
            'ruta desconocida es other' => ['GET', '/no-existe', 'other'],
        ];
    }

    #[DataProvider('rutas')]
    public function testRuta(string $metodo, string $uri, string $esperada): void
    {
        $this->assertSame($esperada, Metricas::ruta($metodo, $uri));
    }

    public function testExposicionAcumulaEntrePeticiones(): void
    {
        $metricas = new Metricas($this->archivo);
        $metricas->registrar('GET', '/departamentos/{id}', 200, 0.003);
        $metricas->registrar('GET', '/departamentos/{id}', 200, 1.5);
        $metricas->registrar('GET', 'other', 404, 0.001);
        // Otra instancia sobre el mismo archivo = otra petición: los contadores se conservan.
        $texto = (new Metricas($this->archivo))->exposicion();

        $this->assertStringContainsString('http_requests_total{method="GET",route="/departamentos/{id}",status="200"} 2', $texto);
        $this->assertStringContainsString('http_requests_total{method="GET",route="other",status="404"} 1', $texto);
        $this->assertStringContainsString('http_request_duration_seconds_bucket{method="GET",route="/departamentos/{id}",status="200",le="0.005"} 1', $texto);
        $this->assertStringContainsString('http_request_duration_seconds_bucket{method="GET",route="/departamentos/{id}",status="200",le="2"} 2', $texto);
        $this->assertStringContainsString('http_request_duration_seconds_bucket{method="GET",route="/departamentos/{id}",status="200",le="+Inf"} 2', $texto);
        $this->assertStringContainsString('http_request_duration_seconds_sum{method="GET",route="/departamentos/{id}",status="200"} 1.503', $texto);
        $this->assertStringContainsString('http_request_duration_seconds_count{method="GET",route="/departamentos/{id}",status="200"} 2', $texto);
        $this->assertStringContainsString('# TYPE http_request_duration_seconds histogram', $texto);
        $this->assertStringContainsString('# TYPE http_requests_total counter', $texto);
    }

    public function testExposicionIncluyeElProceso(): void
    {
        $texto = (new Metricas($this->archivo))->exposicion();

        $this->assertMatchesRegularExpression('/^process_cpu_seconds_total \d/m', $texto);
        $this->assertMatchesRegularExpression('/^process_resident_memory_bytes \d+$/m', $texto);
        $this->assertStringEndsWith("\n", $texto);
    }

    public function testSinArchivoNoRevienta(): void
    {
        $metricas = new Metricas('/ruta/que/no/existe/x.json');
        $metricas->registrar('GET', '/health', 200, 0.1);

        $texto = $metricas->exposicion();

        $this->assertStringContainsString('# TYPE http_requests_total counter', $texto);
        $this->assertStringNotContainsString('http_requests_total{', $texto);
    }

    public function testDesdeEntornoUsaMetricasArchivo(): void
    {
        putenv("METRICAS_ARCHIVO={$this->archivo}");
        try {
            Metricas::desdeEntorno()->registrar('GET', '/health', 200, 0.1);
        } finally {
            putenv('METRICAS_ARCHIVO');
        }

        $this->assertStringContainsString('route="/health"', (new Metricas($this->archivo))->exposicion());
    }
}
