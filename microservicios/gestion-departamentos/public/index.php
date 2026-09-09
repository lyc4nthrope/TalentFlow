<?php

require __DIR__ . '/../src/Database.php';
require __DIR__ . '/../src/DepartamentoRepository.php';
require __DIR__ . '/../src/OpenApi.php';

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

try {
    $repositorio = new DepartamentoRepository(Database::obtenerConexion());
} catch (PDOException $e) {
    http_response_code(500);
    echo json_encode(['error' => 'No fue posible conectar con la base de datos']);
    exit;
}

// --- POST /departamentos ---
if ($metodo === 'POST' && $uri === '/departamentos') {
    $cuerpo = file_get_contents('php://input');
    $datos = json_decode($cuerpo, true);

    if ($cuerpo !== '' && json_last_error() !== JSON_ERROR_NONE) {
        http_response_code(400);
        echo json_encode(['error' => 'Cuerpo JSON inválido']);
        exit;
    }
    $datos = $datos ?? [];

    $faltantes = [];
    foreach (['id', 'nombre'] as $campo) {
        if (empty($datos[$campo])) {
            $faltantes[] = $campo;
        }
    }
    if (!empty($faltantes)) {
        http_response_code(400);
        echo json_encode(['error' => 'Faltan los campos obligatorios: ' . implode(', ', $faltantes)]);
        exit;
    }

    if ($repositorio->buscarPorId($datos['id']) !== null) {
        http_response_code(400);
        echo json_encode(['error' => "El departamento con id {$datos['id']} ya está registrado"]);
        exit;
    }

    try {
        $departamento = $repositorio->guardar($datos);
    } catch (PDOException $e) {
        // Red de seguridad ante condición de carrera: el id es PRIMARY KEY en el esquema,
        // así que una violación de esa restricción (código SQLSTATE 23000) confirma
        // duplicado aunque dos peticiones lleguen casi al mismo tiempo.
        if ($e->getCode() === '23000') {
            http_response_code(400);
            echo json_encode(['error' => "El departamento con id {$datos['id']} ya está registrado"]);
            exit;
        }
        http_response_code(500);
        echo json_encode(['error' => 'Error interno del servidor']);
        exit;
    }

    http_response_code(201);
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
        http_response_code(404);
        echo json_encode(['error' => "El departamento con id {$coincidencias[1]} no existe"]);
        exit;
    }
    echo json_encode($departamento);
    exit;
}

// --- Ruta no soportada ---
http_response_code(404);
echo json_encode(['error' => 'Recurso no encontrado']);