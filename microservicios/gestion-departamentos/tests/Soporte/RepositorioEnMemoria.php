<?php

/** Repositorio falso para probar las reglas sin base de datos. */
final class RepositorioEnMemoria implements RepositorioDepartamentos
{
    /** @var array<string, array> */
    public array $filas = [];
    /** Si se define, guardar() lanza esta excepción (simula la carrera o un fallo de la BD). */
    public ?PDOException $falloAlGuardar = null;

    public function guardar(array $departamento): array
    {
        if ($this->falloAlGuardar !== null) {
            throw $this->falloAlGuardar;
        }
        $id = (string) $departamento['id'];
        $this->filas[$id] = ['id' => $id, 'nombre' => $departamento['nombre'], 'descripcion' => $departamento['descripcion'] ?? null];
        return $this->filas[$id];
    }

    public function buscarPorId(string $id): ?array
    {
        return $this->filas[$id] ?? null;
    }

    public function listar(): array
    {
        return array_values($this->filas);
    }

    public function actualizar(string $id, array $datos): ?array
    {
        if (!isset($this->filas[$id])) {
            return null;
        }
        $this->filas[$id] = ['id' => $id, 'nombre' => $datos['nombre'], 'descripcion' => $datos['descripcion']];
        return $this->filas[$id];
    }

    public function eliminar(string $id): bool
    {
        $existia = isset($this->filas[$id]);
        unset($this->filas[$id]);
        return $existia;
    }

    /** PDOException con un SQLSTATE dado (el constructor solo acepta códigos enteros). */
    public static function errorPdo(string $sqlstate): PDOException
    {
        return new class ($sqlstate) extends PDOException {
            public function __construct(string $sqlstate)
            {
                parent::__construct("SQLSTATE[$sqlstate]");
                $this->code = $sqlstate;
            }
        };
    }
}
