<?php

/**
 * Métricas Prometheus del servicio (Reto 8, D5 y D15). El servidor de PHP ejecuta el script
 * desde cero en cada petición, así que los contadores viven en un archivo JSON protegido con
 * flock. Se pierde al reiniciar el contenedor; rate() de Prometheus maneja ese reinicio.
 * Infraestructura (D7): tiene su prueba en test/test_metricas.php.
 */
final class Metricas
{
    public const RUTA_OTRA = 'other';
    // Hasta 10 s: el umbral de la alerta de latencia (2 s) cae en un bucket exacto.
    public const BUCKETS = [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10];

    public function __construct(private string $archivo)
    {
    }

    public static function desdeEntorno(): self
    {
        return new self(getenv('METRICAS_ARCHIVO') ?: sys_get_temp_dir() . '/talentflow-metricas.json');
    }

    /** Suma una petición a la serie (method, route, status) y su duración al histograma. */
    public function registrar(string $metodo, string $ruta, int $estado, float $segundos): void
    {
        $this->conBloqueo(LOCK_EX, function (array $datos) use ($metodo, $ruta, $estado, $segundos): array {
            $clave = json_encode([$metodo, $ruta, (string) $estado]);
            $serie = $datos[$clave] ?? ['count' => 0, 'sum' => 0.0, 'buckets' => array_fill(0, count(self::BUCKETS), 0)];
            $serie['count']++;
            $serie['sum'] += $segundos;
            foreach (self::BUCKETS as $i => $limite) {
                if ($segundos <= $limite) {
                    $serie['buckets'][$i]++;
                }
            }
            $datos[$clave] = $serie;
            return $datos;
        });
    }

    /** Formato de texto de Prometheus (0.0.4). */
    public function exposicion(): string
    {
        $datos = $this->conBloqueo(LOCK_SH, null);
        $lineas = [
            '# HELP http_requests_total Peticiones HTTP recibidas',
            '# TYPE http_requests_total counter',
        ];
        foreach ($datos as $clave => $serie) {
            $lineas[] = 'http_requests_total{' . self::etiquetas($clave) . '} ' . $serie['count'];
        }
        $lineas[] = '# HELP http_request_duration_seconds Duración de las peticiones HTTP en segundos';
        $lineas[] = '# TYPE http_request_duration_seconds histogram';
        foreach ($datos as $clave => $serie) {
            $etiquetas = self::etiquetas($clave);
            foreach (self::BUCKETS as $i => $limite) {
                $lineas[] = "http_request_duration_seconds_bucket{{$etiquetas},le=\"{$limite}\"} {$serie['buckets'][$i]}";
            }
            $lineas[] = "http_request_duration_seconds_bucket{{$etiquetas},le=\"+Inf\"} {$serie['count']}";
            $lineas[] = "http_request_duration_seconds_sum{{$etiquetas}} {$serie['sum']}";
            $lineas[] = "http_request_duration_seconds_count{{$etiquetas}} {$serie['count']}";
        }
        return implode("\n", array_merge($lineas, self::metricasDeProceso())) . "\n";
    }

    /**
     * Patrón de la ruta y no la URL, para no crear una serie por cada id. Replica el
     * enrutamiento de public/index.php: lo que no casa con una ruta es "other".
     */
    public static function ruta(string $metodo, string $uri): string
    {
        $fijas = [
            'GET /health' => '/health',
            'GET /departamentos' => '/departamentos',
            'POST /departamentos' => '/departamentos',
            'GET /departamentos/docs' => '/departamentos/docs',
            'GET /departamentos/openapi.json' => '/departamentos/openapi.json',
        ];
        if (isset($fijas["$metodo $uri"])) {
            return $fijas["$metodo $uri"];
        }
        if ($metodo === 'GET' && preg_match('#^/departamentos/[^/]+$#', $uri)) {
            return '/departamentos/{id}';
        }
        return self::RUTA_OTRA;
    }

    /** CPU y memoria del proceso del servidor (el mismo para todas las peticiones). */
    private static function metricasDeProceso(): array
    {
        $uso = getrusage();
        $cpu = $uso['ru_utime.tv_sec'] + $uso['ru_utime.tv_usec'] / 1e6
            + $uso['ru_stime.tv_sec'] + $uso['ru_stime.tv_usec'] / 1e6;
        $lineas = [
            '# HELP process_cpu_seconds_total Tiempo de CPU de usuario y sistema en segundos',
            '# TYPE process_cpu_seconds_total counter',
            "process_cpu_seconds_total {$cpu}",
        ];
        $estado = @file_get_contents('/proc/self/status');
        if ($estado !== false && preg_match('/^VmRSS:\s+(\d+) kB/m', $estado, $m)) {
            $lineas[] = '# HELP process_resident_memory_bytes Memoria residente en bytes';
            $lineas[] = '# TYPE process_resident_memory_bytes gauge';
            $lineas[] = 'process_resident_memory_bytes ' . ((int) $m[1] * 1024);
        }
        return $lineas;
    }

    private static function etiquetas(string $clave): string
    {
        [$metodo, $ruta, $estado] = json_decode($clave, true);
        return sprintf('method="%s",route="%s",status="%s"', $metodo, addcslashes($ruta, "\"\\"), $estado);
    }

    /** Lee (y si hay $cambio, reescribe) el archivo con flock: sin escrituras a medias. */
    private function conBloqueo(int $modo, ?callable $cambio): array
    {
        $fp = @fopen($this->archivo, 'c+');
        if ($fp === false) {
            return [];
        }
        try {
            flock($fp, $modo);
            $datos = json_decode(stream_get_contents($fp) ?: '[]', true) ?: [];
            if ($cambio !== null) {
                $datos = $cambio($datos);
                ftruncate($fp, 0);
                rewind($fp);
                fwrite($fp, json_encode($datos));
                fflush($fp);
            }
            return $datos;
        } finally {
            flock($fp, LOCK_UN);
            fclose($fp);
        }
    }
}
