<?php

/** Puerto de persistencia: lo implementa DepartamentoRepository (PDO) y un falso en las pruebas. */
interface RepositorioDepartamentos
{
    public function guardar(array $departamento): array;

    public function buscarPorId(string $id): ?array;

    public function listar(): array;

    /** El departamento actualizado, o null si no existe. */
    public function actualizar(string $id, array $datos): ?array;

    /** true si borró una fila. */
    public function eliminar(string $id): bool;
}
