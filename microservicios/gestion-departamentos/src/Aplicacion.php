<?php

/**
 * Adaptador HTTP: método + ruta + cuerpo -> Respuesta, sin globales ni salida. El servicio
 * se crea con una fábrica perezosa para que /health y la documentación respondan aunque la
 * base de datos esté caída.
 */
final class Aplicacion
{
    private const FRASES_ESTADO = [
        400 => 'Bad Request',
        404 => 'Not Found',
        500 => 'Internal Server Error',
    ];
    private const RUTA_CON_ID = '#^/departamentos/([^/]+)$#';

    /** @var callable(): DepartamentoService */
    private $fabricaServicio;
    /** @var callable(): bool */
    private $baseDeDatosDisponible;

    public function __construct(callable $fabricaServicio, callable $baseDeDatosDisponible)
    {
        $this->fabricaServicio = $fabricaServicio;
        $this->baseDeDatosDisponible = $baseDeDatosDisponible;
    }

    public function atender(string $metodo, string $ruta, string $cuerpo = ''): Respuesta
    {
        // Documentación: bajo el prefijo del servicio (el Gateway solo enruta /departamentos/*)
        // y antes de GET /departamentos/{id}, que si no tomaría "docs" como un id.
        if ($metodo === 'GET' && $ruta === '/departamentos/docs') {
            return new Respuesta(200, ['Content-Type' => 'text/html; charset=utf-8'], obtenerHtmlSwagger());
        }
        if ($metodo === 'GET' && $ruta === '/departamentos/openapi.json') {
            return Respuesta::json(200, obtenerEspecificacionOpenApi());
        }
        if ($metodo === 'GET' && $ruta === '/health') {
            return $this->salud();
        }

        try {
            $servicio = ($this->fabricaServicio)();
        } catch (PDOException) {
            return self::error(500, 'No fue posible conectar con la base de datos', $ruta);
        }

        try {
            return $this->enrutar($servicio, $metodo, $ruta, $cuerpo);
        } catch (ErrorDeNegocio $e) {
            return self::error($e->status, $e->getMessage(), $ruta, $e->errores);
        } catch (PDOException) {
            return self::error(500, 'Error interno del servidor', $ruta);
        }
    }

    private function enrutar(DepartamentoService $servicio, string $metodo, string $ruta, string $cuerpo): Respuesta
    {
        if ($ruta === '/departamentos') {
            if ($metodo === 'POST') {
                $departamento = $servicio->crear(self::leerJson($cuerpo));
                return Respuesta::json(201, $departamento, ['Location' => "/departamentos/{$departamento['id']}"]);
            }
            if ($metodo === 'GET') {
                return Respuesta::json(200, $servicio->listar());
            }
        }

        if (preg_match(self::RUTA_CON_ID, $ruta, $coincidencias)) {
            // parse_url no decodifica: "RRHH%201" es el id "RRHH 1".
            $id = rawurldecode($coincidencias[1]);
            switch ($metodo) {
                case 'GET':
                    return Respuesta::json(200, $servicio->obtener($id));
                case 'PUT':
                    return Respuesta::json(200, $servicio->actualizar($id, self::leerJson($cuerpo)));
                case 'DELETE':
                    $servicio->eliminar($id);
                    return new Respuesta(204);
            }
        }

        return self::error(404, 'Recurso no encontrado', $ruta);
    }

    private function salud(): Respuesta
    {
        $estado = ($this->baseDeDatosDisponible)() ? 'UP' : 'DOWN';

        return Respuesta::json($estado === 'UP' ? 200 : 503, [
            'status' => $estado,
            'service' => 'departamentos-service',
            'timestamp' => gmdate('Y-m-d\TH:i:s\Z'),
            // Formato del PDF (Reto 8, D4); "components" se conserva para el Gateway (Reto 3).
            // Este servicio no usa el broker: solo informa la base de datos.
            'checks' => ['database' => $estado],
            'components' => ['app' => 'UP', 'db' => $estado],
        ]);
    }

    /** Cuerpo vacío o JSON que no es objeto: datos vacíos (la validación dirá qué falta). */
    private static function leerJson(string $cuerpo): array
    {
        $datos = json_decode($cuerpo, true);
        if ($cuerpo !== '' && json_last_error() !== JSON_ERROR_NONE) {
            throw new ErrorDeNegocio(400, 'Cuerpo JSON inválido');
        }
        return is_array($datos) ? $datos : [];
    }

    private static function error(int $status, string $mensaje, string $ruta, array $errores = []): Respuesta
    {
        $cuerpo = [
            'status' => $status,
            'error' => self::FRASES_ESTADO[$status] ?? 'Error',
            'message' => $mensaje,
            'timestamp' => gmdate('Y-m-d\TH:i:s\Z'),
            'path' => $ruta,
        ];
        if (!empty($errores)) {
            $cuerpo['errors'] = $errores;
        }
        return Respuesta::json($status, $cuerpo);
    }
}
