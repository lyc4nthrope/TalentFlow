<?php

class DepartamentoRepository implements RepositorioDepartamentos
{
    private PDO $conexion;

    public function __construct(PDO $conexion)
    {
        $this->conexion = $conexion;
    }

    public function guardar(array $departamento): array
    {
        $sentencia = $this->conexion->prepare(
            'INSERT INTO departamentos (id, nombre, descripcion) VALUES (:id, :nombre, :descripcion)'
        );
        $sentencia->execute([
            'id' => $departamento['id'],
            'nombre' => $departamento['nombre'],
            'descripcion' => $departamento['descripcion'] ?? null,
        ]);

        return $this->buscarPorId($departamento['id']);
    }

    public function buscarPorId(string $id): ?array
    {
        $sentencia = $this->conexion->prepare(
            'SELECT id, nombre, descripcion FROM departamentos WHERE id = :id'
        );
        $sentencia->execute(['id' => $id]);
        $fila = $sentencia->fetch();

        return $fila ?: null;
    }

    public function listar(): array
    {
        $sentencia = $this->conexion->query(
            'SELECT id, nombre, descripcion FROM departamentos ORDER BY creado_en ASC, id ASC'
        );

        return $sentencia->fetchAll();
    }

    public function actualizar(string $id, array $datos): ?array
    {
        // Sin rowCount(): MySQL cuenta 0 filas si los valores no cambian.
        $sentencia = $this->conexion->prepare(
            'UPDATE departamentos SET nombre = :nombre, descripcion = :descripcion WHERE id = :id'
        );
        $sentencia->execute([
            'id' => $id,
            'nombre' => $datos['nombre'],
            'descripcion' => $datos['descripcion'] ?? null,
        ]);

        return $this->buscarPorId($id);
    }

    public function eliminar(string $id): bool
    {
        $sentencia = $this->conexion->prepare('DELETE FROM departamentos WHERE id = :id');
        $sentencia->execute(['id' => $id]);

        return $sentencia->rowCount() > 0;
    }
}
