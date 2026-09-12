<?php

/**
 * Pruebas de integración (black-box, vía HTTP) para el microservicio de departamentos.
 * Sin dependencias externas: solo usa file_get_contents/stream_context_create.
 *
 * Cubre exactamente lo exigido en el documento del Reto 2:
 *   - POST /departamentos -> 201 Created con el departamento creado
 *   - GET  /departamentos/{id} -> 200 OK con la info del departamento
 *   - GET  /departamentos/{id} inexistente -> 404 Not Found con mensaje descriptivo
 *   - GET  /departamentos -> lista todos los departamentos
 *   - Estructura del departamento: id, nombre, descripcion
 *
 * Y, aparte, una sección de "extra" para comportamientos que el código ya
 * implementa pero que el documento del reto no exige explícitamente
 * (validación de campos obligatorios y duplicado de id con 400).
 *
 * Uso:
 *   php test_departamentos.php
 *   TEST_BASE_URL=http://localhost:8081 php test_departamentos.php
 */

$baseUrl = getenv('TEST_BASE_URL') ?: 'http://localhost:8081';

$totalPruebas = 0;
$totalFallos = 0;

function peticion(string $metodo, string $url, ?array $cuerpo = null): array
{
    $opciones = [
        'http' => [
            'method' => $metodo,
            'header' => "Content-Type: application/json\r\n",
            'ignore_errors' => true, // para poder leer el body en respuestas 4xx/5xx
        ],
    ];

    if ($cuerpo !== null) {
        $opciones['http']['content'] = json_encode($cuerpo);
    }

    $contexto = stream_context_create($opciones);
    $respuestaCruda = @file_get_contents($url, false, $contexto);

    $status = 0;
    foreach ($http_response_header ?? [] as $header) {
        if (preg_match('#^HTTP/\S+\s+(\d{3})#', $header, $coincidencias)) {
            $status = (int) $coincidencias[1];
        }
    }

    $json = $respuestaCruda !== false ? json_decode($respuestaCruda, true) : null;

    return ['status' => $status, 'body' => $json, 'raw' => $respuestaCruda];
}

function verificar(string $descripcion, bool $condicion, string $detalle = ''): void
{
    global $totalPruebas, $totalFallos;
    $totalPruebas++;

    if ($condicion) {
        echo "  [OK] {$descripcion}\n";
    } else {
        $totalFallos++;
        echo "  [FALLA] {$descripcion}" . ($detalle ? " -> {$detalle}" : '') . "\n";
    }
}

// Id único por corrida para no chocar con datos de ejecuciones anteriores
// (el servicio no expone un DELETE, así que los datos de prueba persisten en la BD)
$idPrueba = 'TST_' . substr(md5((string) microtime(true)), 0, 8);
$idInexistente = 'NOEXISTE_' . substr(md5((string) microtime(true) . 'x'), 0, 8);

echo "== Pruebas del Servicio de Departamentos ({$baseUrl}) ==\n\n";

// --- Requisito: POST /departamentos registra un nuevo departamento -> 201 ---
echo "POST /departamentos (registro exitoso)\n";
$respuesta = peticion('POST', "{$baseUrl}/departamentos", [
    'id' => $idPrueba,
    'nombre' => 'Departamento de Prueba',
    'descripcion' => 'Creado por el script de pruebas automatizadas',
]);
verificar('Responde 201 Created', $respuesta['status'] === 201, "status recibido: {$respuesta['status']}");
verificar('El body incluye el id enviado', ($respuesta['body']['id'] ?? null) === $idPrueba);
verificar('El body incluye el nombre enviado', ($respuesta['body']['nombre'] ?? null) === 'Departamento de Prueba');
verificar('El body incluye la descripción enviada', ($respuesta['body']['descripcion'] ?? null) === 'Creado por el script de pruebas automatizadas');
echo "\n";

// --- Requisito: GET /departamentos/{id} consulta un departamento existente -> 200 ---
echo "GET /departamentos/{id} (consulta exitosa)\n";
$respuesta = peticion('GET', "{$baseUrl}/departamentos/{$idPrueba}");
verificar('Responde 200 OK', $respuesta['status'] === 200, "status recibido: {$respuesta['status']}");
verificar('Devuelve el id correcto', ($respuesta['body']['id'] ?? null) === $idPrueba);
verificar('Devuelve el nombre correcto', ($respuesta['body']['nombre'] ?? null) === 'Departamento de Prueba');
echo "\n";

// --- Requisito: GET /departamentos/{id} inexistente -> 404 con mensaje descriptivo ---
echo "GET /departamentos/{id} (departamento inexistente)\n";
$respuesta = peticion('GET', "{$baseUrl}/departamentos/{$idInexistente}");
verificar('Responde 404 Not Found', $respuesta['status'] === 404, "status recibido: {$respuesta['status']}");
verificar(
    'El mensaje es descriptivo (menciona el id consultado)',
    isset($respuesta['body']['message']) && str_contains($respuesta['body']['message'], $idInexistente),
    'mensaje recibido: ' . ($respuesta['body']['message'] ?? 'null')
);
echo "\n";

// --- Requisito: GET /departamentos lista todos los departamentos ---
echo "GET /departamentos (listado)\n";
$respuesta = peticion('GET', "{$baseUrl}/departamentos");
verificar('Responde 200 OK', $respuesta['status'] === 200, "status recibido: {$respuesta['status']}");
verificar('El body es un arreglo', is_array($respuesta['body']));
$idsEnListado = array_column($respuesta['body'] ?? [], 'id');
verificar('El departamento recién creado aparece en el listado', in_array($idPrueba, $idsEnListado, true));
echo "\n";

// --- Requisito: estructura del departamento (id, nombre, descripcion) ---
echo "Estructura del departamento\n";
$respuesta = peticion('GET', "{$baseUrl}/departamentos/{$idPrueba}");
$claves = array_keys($respuesta['body'] ?? []);
verificar('Tiene la clave "id"', in_array('id', $claves, true));
verificar('Tiene la clave "nombre"', in_array('nombre', $claves, true));
verificar('Tiene la clave "descripcion"', in_array('descripcion', $claves, true));
echo "\n";

// =========================================================================
// EXTRA: comportamientos que el código ya implementa pero que el documento
// del Reto 2 no exige explícitamente. Se dejan aparte para que quede claro
// qué corresponde al enunciado y qué es criterio propio del equipo.
// =========================================================================
echo "-- EXTRA (no exigido explícitamente por el documento del reto) --\n\n";

echo "POST /departamentos con id duplicado\n";
$respuesta = peticion('POST', "{$baseUrl}/departamentos", [
    'id' => $idPrueba, // mismo id ya creado arriba
    'nombre' => 'Duplicado',
    'descripcion' => 'No debería crearse',
]);
verificar('Responde 400 Bad Request', $respuesta['status'] === 400, "status recibido: {$respuesta['status']}");
echo "\n";

echo "POST /departamentos sin campos obligatorios\n";
$respuesta = peticion('POST', "{$baseUrl}/departamentos", [
    'descripcion' => 'Falta id y nombre',
]);
verificar('Responde 400 Bad Request', $respuesta['status'] === 400, "status recibido: {$respuesta['status']}");
echo "\n";

// --- Resumen ---
echo "== Resumen ==\n";
echo "Total de verificaciones: {$totalPruebas}\n";
echo "Fallidas: {$totalFallos}\n";

if ($totalFallos > 0) {
    echo "\nRESULTADO: FALLÓ\n";
    exit(1);
}

echo "\nRESULTADO: OK\n";
exit(0);