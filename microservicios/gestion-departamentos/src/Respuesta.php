<?php

/** Respuesta HTTP como valor: public/index.php la emite y las pruebas la inspeccionan. */
final class Respuesta
{
    /** @param array<string, string> $cabeceras */
    public function __construct(
        public readonly int $status,
        public readonly array $cabeceras = [],
        public readonly string $cuerpo = '',
    ) {
    }

    public static function json(int $status, mixed $datos, array $cabeceras = []): self
    {
        return new self($status, ['Content-Type' => 'application/json; charset=utf-8'] + $cabeceras, json_encode($datos));
    }
}
