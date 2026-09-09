<?php

function obtenerEspecificacionOpenApi(): array
{
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
            'error' => ['type' => 'string'],
        ],
    ];

    return [
        'openapi' => '3.0.3',
        'info' => [
            'title' => 'TalentFlow - Servicio de Gestión de Departamentos',
            'version' => '0.1.0',
            'description' => 'Registro, consulta y listado de departamentos. Persiste en MySQL.',
        ],
        'servers' => [['url' => '/']],
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
                    'parameters' => [
                        ['name' => 'id', 'in' => 'path', 'required' => true, 'schema' => ['type' => 'string'], 'example' => 'IT'],
                    ],
                    'responses' => [
                        '200' => [
                            'description' => 'Departamento encontrado',
                            'content' => ['application/json' => ['schema' => ['$ref' => '#/components/schemas/Departamento']]],
                        ],
                        '404' => [
                            'description' => 'El departamento no existe',
                            'content' => ['application/json' => ['schema' => ['$ref' => '#/components/schemas/Error']]],
                        ],
                    ],
                ],
            ],
        ],
        'components' => [
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
        url: '/openapi.json',
        dom_id: '#swagger-ui'
      });
    };
  </script>
</body>
</html>
HTML;
}