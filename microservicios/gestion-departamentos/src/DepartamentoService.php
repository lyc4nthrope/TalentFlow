<?php

/** Reglas de negocio de los departamentos; no sabe nada de HTTP. */
final class DepartamentoService
{
    /** Largo máximo de cada columna en init.sql: lo que pase de ahí MySQL lo rechaza con un 500. */
    private const LARGO_MAXIMO = ['id' => 20, 'nombre' => 100, 'descripcion' => 255];

    public function __construct(private RepositorioDepartamentos $repositorio)
    {
    }

    public function crear(array $datos): array
    {
        self::exigirCampos($datos, ['id', 'nombre']);
        self::validarFormato($datos);
        $id = (string) $datos['id'];

        if ($this->repositorio->buscarPorId($id) !== null) {
            throw self::idDuplicado($datos['id']);
        }

        try {
            return $this->repositorio->guardar($datos);
        } catch (PDOException $e) {
            // Red de seguridad ante condición de carrera: el id es PRIMARY KEY, así que una
            // violación de esa restricción (SQLSTATE 23000) confirma el duplicado.
            if ($e->getCode() === '23000') {
                throw self::idDuplicado($datos['id']);
            }
            throw $e;
        }
    }

    public function listar(): array
    {
        return $this->repositorio->listar();
    }

    public function obtener(string $id): array
    {
        return $this->repositorio->buscarPorId($id) ?? throw self::noExiste($id);
    }

    /** Reemplazo completo: descripcion ausente queda en null. El id no se cambia. */
    public function actualizar(string $id, array $datos): array
    {
        self::exigirCampos($datos, ['nombre']);
        self::validarFormato($datos);
        if (array_key_exists('id', $datos) && (string) $datos['id'] !== $id) {
            throw new ErrorDeNegocio(
                400,
                'No se puede cambiar el id del departamento',
                [['field' => 'id', 'message' => 'No se puede cambiar el id', 'rejectedValue' => $datos['id']]]
            );
        }
        $cambios = ['nombre' => $datos['nombre'], 'descripcion' => $datos['descripcion'] ?? null];
        // El repositorio devuelve null si el id no existe: un solo viaje, sin carrera.
        return $this->repositorio->actualizar($id, $cambios) ?? throw self::noExiste($id);
    }

    public function eliminar(string $id): void
    {
        if (!$this->repositorio->eliminar($id)) {
            throw self::noExiste($id);
        }
    }

    private static function exigirCampos(array $datos, array $campos): void
    {
        $faltantes = array_values(array_filter($campos, fn ($campo) => empty($datos[$campo])));
        if (!empty($faltantes)) {
            throw new ErrorDeNegocio(
                400,
                'Faltan los campos obligatorios: ' . implode(', ', $faltantes),
                array_map(fn ($campo) => ['field' => $campo, 'message' => 'Es un campo obligatorio'], $faltantes)
            );
        }
    }

    /** Texto (o número, como antes) y dentro del largo de la columna, contado en caracteres. */
    private static function validarFormato(array $datos): void
    {
        $errores = [];
        foreach (self::LARGO_MAXIMO as $campo => $maximo) {
            $valor = $datos[$campo] ?? null;
            if ($valor === null) {
                continue;
            }
            if (!is_string($valor) && !is_int($valor)) {
                $errores[] = ['field' => $campo, 'message' => 'Debe ser texto'];
            } elseif (mb_strlen((string) $valor) > $maximo) {
                $errores[] = ['field' => $campo, 'message' => "Máximo {$maximo} caracteres"];
            }
        }
        if (!empty($errores)) {
            throw new ErrorDeNegocio(
                400,
                'Campos con formato inválido: ' . implode(', ', array_column($errores, 'field')),
                $errores
            );
        }
    }

    private static function idDuplicado(mixed $id): ErrorDeNegocio
    {
        return new ErrorDeNegocio(
            400,
            "El departamento con id {$id} ya está registrado",
            [['field' => 'id', 'message' => 'Ya está registrado', 'rejectedValue' => $id]]
        );
    }

    private static function noExiste(string $id): ErrorDeNegocio
    {
        return new ErrorDeNegocio(404, "El departamento con id {$id} no existe");
    }
}
