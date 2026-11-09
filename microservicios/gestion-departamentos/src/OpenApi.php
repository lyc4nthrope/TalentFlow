<?php

function obtenerEspecificacionOpenApi(): array
{
    $parametroId = ['name' => 'id', 'in' => 'path', 'required' => true, 'schema' => ['type' => 'string'], 'example' => 'IT'];
    $error = fn (string $descripcion) => [
        'description' => $descripcion,
        'content' => ['application/json' => ['schema' => ['$ref' => '#/components/schemas/Error']]],
    ];

    $departamentoSchema = [
        'type' => 'object',
        'properties' => [
            'id' => ['type' => 'string', 'example' => 'IT'],
            'nombre' => ['type' => 'string', 'example' => 'Tecnología'],
            'descripcion' => ['type' => 'string', 'example' => 'Departamento de TI'],
        ],
    ];

    $errorSchema = [
        'type' => 'object',
        'properties' => [
            'status' => ['type' => 'integer', 'example' => 400],
            'error' => ['type' => 'string', 'example' => 'Bad Request'],
            'message' => ['type' => 'string', 'example' => 'El departamento con id IT ya está registrado'],
            'timestamp' => ['type' => 'string', 'format' => 'date-time'],
            'path' => ['type' => 'string', 'example' => '/departamentos'],
            'errors' => [
                'type' => 'array',
                'items' => [
                    'type' => 'object',
                    'properties' => [
                        'field' => ['type' => 'string'],
                        'message' => ['type' => 'string'],
                        'rejectedValue' => [],
                    ],
                ],
            ],
        ],
    ];

    return [
        'openapi' => '3.0.3',
        'info' => [
            'title' => 'TalentFlow - Servicio de Gestión de Departamentos',
            'version' => '0.1.0',
            'description' => 'Registro, consulta, listado, actualización y eliminación de departamentos. Persiste en MySQL.',
        ],
        'servers' => [['url' => '/']],
        // D13: el Gateway exige un JWT. Los [] vacíos son listas PHP y json_encode los
        // emite como [] (no {}), que es lo que pide OpenAPI para un requisito sin scopes.
        'security' => [['bearerAuth' => []]],
        'paths' => [
            '/departamentos' => [
                'post' => [
                    'summary' => 'Registra un nuevo departamento',
                    'requestBody' => [
                        'required' => true,
                        'content' => [
                            'application/json' => [
                                'schema' => [
                                    'allOf' => [
                                        ['$ref' => '#/components/schemas/Departamento'],
                                        ['required' => ['id', 'nombre']],
                                    ],
                                ],
                            ],
                        ],
                    ],
                    'responses' => [
                        '201' => [
                            'description' => 'Departamento registrado',
                            'headers' => [
                                'Location' => [
                                    'description' => 'URL del departamento creado',
                                    'schema' => ['type' => 'string', 'example' => '/departamentos/IT'],
                                ],
                            ],
                            'content' => ['application/json' => ['schema' => ['$ref' => '#/components/schemas/Departamento']]],
                        ],
                        '400' => [
                            'description' => 'Id ya registrado o campos obligatorios faltantes',
                            'content' => ['application/json' => ['schema' => ['$ref' => '#/components/schemas/Error']]],
                        ],
                    ],
                ],
                'get' => [
                    'summary' => 'Lista todos los departamentos registrados',
                    'responses' => [
                        '200' => [
                            'description' => 'Listado de departamentos',
                            'content' => [
                                'application/json' => [
                                    'schema' => ['type' => 'array', 'items' => ['$ref' => '#/components/schemas/Departamento']],
                                ],
                            ],
                        ],
                    ],
                ],
            ],
            '/departamentos/{id}' => [
                'get' => [
                    'summary' => 'Consulta un departamento por id',
                    'parameters' => [$parametroId],
                    'responses' => [
                        '200' => [
                            'description' => 'Departamento encontrado',
                            'content' => ['application/json' => ['schema' => ['$ref' => '#/components/schemas/Departamento']]],
                        ],
                        '404' => $error('El departamento no existe'),
                    ],
                ],
                'put' => [
                    'summary' => 'Actualiza el nombre y la descripción de un departamento',
                    'description' => 'Reemplazo completo: si no se envía descripcion queda en null. El id no se puede cambiar.',
                    'parameters' => [$parametroId],
                    'requestBody' => [
                        'required' => true,
                        'content' => [
                            'application/json' => [
                                'schema' => [
                                    'type' => 'object',
                                    'required' => ['nombre'],
                                    'properties' => [
                                        'nombre' => ['type' => 'string', 'example' => 'Tecnología e Innovación'],
                                        'descripcion' => ['type' => 'string', 'example' => 'Departamento de TI'],
                                    ],
                                ],
                            ],
                        ],
                    ],
                    'responses' => [
                        '200' => [
                            'description' => 'Departamento actualizado',
                            'content' => ['application/json' => ['schema' => ['$ref' => '#/components/schemas/Departamento']]],
                        ],
                        '400' => $error('Falta el nombre, cuerpo JSON inválido o se intentó cambiar el id'),
                        '404' => $error('El departamento no existe'),
                    ],
                ],
                'delete' => [
                    'summary' => 'Elimina un departamento',
                    'description' => 'Los empleados ya asignados conservan su departamentoId; '
                        . 'registrar un empleado nuevo en un departamento eliminado responde 400.',
                    'parameters' => [$parametroId],
                    'responses' => [
                        '204' => ['description' => 'Departamento eliminado'],
                        '404' => $error('El departamento no existe'),
                    ],
                ],
            ],
        ],
        'components' => [
            'securitySchemes' => [
                'bearerAuth' => ['type' => 'http', 'scheme' => 'bearer', 'bearerFormat' => 'JWT'],
            ],
            'schemas' => [
                'Departamento' => $departamentoSchema,
                'Error' => $errorSchema,
            ],
        ],
    ];
}

function obtenerHtmlSwagger(): string
{
    return <<<HTML
<!DOCTYPE html>
<html>
<head>
  <title>TalentFlow - Departamentos - Docs</title>
  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.11.0/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.11.0/swagger-ui-bundle.js"></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: '/departamentos/openapi.json',
        dom_id: '#swagger-ui',
        persistAuthorization: true
      });
    };
  </script>
</body>
</html>
HTML;
}