<?php

require __DIR__ . '/../src/Database.php';
require __DIR__ . '/../src/DepartamentoRepository.php';
require __DIR__ . '/../src/OpenApi.php';

const FRASES_ESTADO = [
    400 => 'Bad Request',
    404 => 'Not Found',
    500 => 'Internal Server Error',
];

function responderError(int $status, string $mensaje, array $errores = []): void
{
    global $uri;

    http_response_code($status);
    $cuerpo = [
        'status' => $status,
        'error' => FRASES_ESTADO[$status] ?? 'Error',
        'message' => $mensaje,
        'timestamp' => gmdate('Y-m-d\TH:i:s\Z'),
        'path' => $uri,
    ];
    if (!empty($errores)) {
        $cuerpo['errors'] = $errores;
    }
    echo json_encode($cuerpo);
    exit;
}

$metodo = $_SERVER['REQUEST_METHOD'];
$uri = parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH);

// --- Documentación (no requieren base de datos) ---
if ($metodo === 'GET' && $uri === '/docs') {
    header('Content-Type: text/html; charset=utf-8');
    echo obtenerHtmlSwagger();
    exit;
}

if ($metodo === 'GET' && $uri === '/openapi.json') {
    header('Content-Type: application/json; charset=utf-8');
    echo json_encode(obtenerEspecificacionOpenApi());
    exit;
}

header('Content-Type: application/json; charset=utf-8');

// --- Health check (antes de instanciar el repositorio: debe seguir funcionando aunque la BD falle) ---
if ($metodo === 'GET' && $uri === '/health') {
    $dbStatus = 'UP';
    try {
        Database::obtenerConexion()->query('SELECT 1');
    } catch (PDOException $e) {
        $dbStatus = 'DOWN';
    }

    $statusGeneral = $dbStatus === 'UP' ? 'UP' : 'DOWN';

    http_response_code($statusGeneral === 'UP' ? 200 : 503);
    echo json_encode([
        'status' => $statusGeneral,
        'timestamp' => gmdate('Y-m-d\TH:i:s\Z'),
        'components' => [
            'app' => 'UP',
            'db' => $dbStatus,
        ],
    ]);
    exit;
}

// --- Instanciar el repositorio para las rutas de negocio ---
// (no se necesita en /docs, /openapi.json ni /health, por eso va después de esos)
try {
    $repositorio = new DepartamentoRepository(Database::obtenerConexion());
} catch (PDOException $e) {
    responderError(500, 'No fue posible conectar con la base de datos');
}

// --- POST /departamentos ---
if ($metodo === 'POST' && $uri === '/departamentos') {
    $cuerpo = file_get_contents('php://input');
    $datos = json_decode($cuerpo, true);

    if ($cuerpo !== '' && json_last_error() !== JSON_ERROR_NONE) {
        responderError(400, 'Cuerpo JSON inválido');
    }
    $datos = $datos ?? [];

    $faltantes = [];
    foreach (['id', 'nombre'] as $campo) {
        if (empty($datos[$campo])) {
            $faltantes[] = $campo;
        }
    }
    if (!empty($faltantes)) {
        responderError(
            400,
            'Faltan los campos obligatorios: ' . implode(', ', $faltantes),
            array_map(fn ($campo) => ['field' => $campo, 'message' => 'Es un campo obligatorio'], $faltantes)
        );
    }

    if ($repositorio->buscarPorId($datos['id']) !== null) {
        responderError(
            400,
            "El departamento con id {$datos['id']} ya está registrado",
            [['field' => 'id', 'message' => 'Ya está registrado', 'rejectedValue' => $datos['id']]]
        );
    }

    try {
        $departamento = $repositorio->guardar($datos);
    } catch (PDOException $e) {
        // Red de seguridad ante condición de carrera: el id es PRIMARY KEY en el esquema,
        // así que una violación de esa restricción (código SQLSTATE 23000) confirma
        // duplicado aunque dos peticiones lleguen casi al mismo tiempo.
        if ($e->getCode() === '23000') {
            responderError(
                400,
                "El departamento con id {$datos['id']} ya está registrado",
                [['field' => 'id', 'message' => 'Ya está registrado', 'rejectedValue' => $datos['id']]]
            );
        }
        responderError(500, 'Error interno del servidor');
    }

    http_response_code(201);
    header("Location: /departamentos/{$departamento['id']}");
    echo json_encode($departamento);
    exit;
}

// --- GET /departamentos (listar) ---
if ($metodo === 'GET' && $uri === '/departamentos') {
    echo json_encode($repositorio->listar());
    exit;
}

// --- GET /departamentos/{id} ---
if ($metodo === 'GET' && preg_match('#^/departamentos/([^/]+)$#', $uri, $coincidencias)) {
    $departamento = $repositorio->buscarPorId($coincidencias[1]);
    if ($departamento === null) {
        responderError(404, "El departamento con id {$coincidencias[1]} no existe");
    }
    echo json_encode($departamento);
    exit;
}

// --- Ruta no soportada ---
responderError(404, 'Recurso no encontrado');