<?php

// Punto de entrada: métricas, trazas, logs y caos; el enrutamiento vive en src/Aplicacion.php.
require __DIR__ . '/../src/cargar.php';

$metodo = $_SERVER['REQUEST_METHOD'];
$uri = parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH);

// --- Métricas (Reto 8, D5): GET /metrics no se cuenta; el resto se mide al terminar ---
$metricas = Metricas::desdeEntorno();
if ($metodo === 'GET' && $uri === '/metrics') {
    header('Content-Type: text/plain; version=0.0.4; charset=utf-8');
    echo $metricas->exposicion();
    exit;
}
$inicio = hrtime(true);
$trazas = Trazas::desdeEntorno($_SERVER['HTTP_TRACEPARENT'] ?? null);
// Corre también tras cada exit: registra el código que de verdad se respondió.
register_shutdown_function(function () use ($metricas, $trazas, $metodo, $uri, $inicio): void {
    $error = error_get_last();
    $fatal = $error !== null && ($error['type'] & (E_ERROR | E_PARSE | E_CORE_ERROR | E_COMPILE_ERROR)) !== 0;
    $estado = $fatal ? 500 : (int) http_response_code();
    $ruta = Metricas::ruta($metodo, $uri);
    $metricas->registrar($metodo, $ruta, $estado, (hrtime(true) - $inicio) / 1e9);
    // El healthcheck de Docker (cada 5 s) no genera trazas: taparía las de negocio.
    if ($ruta !== '/health') {
        $fin = microtime(true);
        $trazas->terminar($metodo, $ruta, $uri, $estado, $_SERVER['REQUEST_TIME_FLOAT'], $fin);
        $duracionMs = round(($fin - $_SERVER['REQUEST_TIME_FLOAT']) * 1000, 1);
        file_put_contents('php://stderr', $trazas->lineaLog(
            $estado >= 500 ? 'ERROR' : 'INFO',
            "$metodo $uri $estado",
            ['route' => $ruta, 'status' => $estado, 'durationMs' => $duracionMs],
        ) . "\n");
    }
});

// --- Simulación de caos (Reto 8, F11): retardo artificial, apagado por defecto ---
// Solo se activa con docker-compose.caos.yml (CAOS_LATENCIA_MS > 0). Va después de iniciar
// métricas y trazas para que el retardo se vea en Grafana y en Jaeger; /health queda fuera
// para que Docker no dé el contenedor por caído (el caos es lentitud, no caída).
$caosMs = (int) (getenv('CAOS_LATENCIA_MS') ?: 0);
$caosProbabilidad = (float) (getenv('CAOS_PROBABILIDAD') ?: 1);
if ($caosMs > 0 && $uri !== '/health' && mt_rand() / mt_getrandmax() < $caosProbabilidad) {
    usleep($caosMs * 1000);
}

// --- Aplicación: el servicio (y la conexión) se crea solo si la ruta lo necesita ---
$aplicacion = new Aplicacion(
    fn () => new DepartamentoService(new DepartamentoRepository(Database::obtenerConexion())),
    function (): bool {
        try {
            Database::obtenerConexion()->query('SELECT 1');
            return true;
        } catch (PDOException) {
            return false;
        }
    },
);
$respuesta = $aplicacion->atender($metodo, $uri, (string) file_get_contents('php://input'));

http_response_code($respuesta->status);
foreach ($respuesta->cabeceras as $nombre => $valor) {
    header("$nombre: $valor");
}
// 204 va sin cuerpo.
if ($respuesta->status !== 204) {
    echo $respuesta->cuerpo;
}
