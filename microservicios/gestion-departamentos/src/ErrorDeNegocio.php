<?php

/** Regla de negocio incumplida: el adaptador HTTP la responde con su status y errores. */
final class ErrorDeNegocio extends RuntimeException
{
    /** @param list<array{field: string, message: string, rejectedValue?: mixed}> $errores */
    public function __construct(
        public readonly int $status,
        string $mensaje,
        public readonly array $errores = [],
    ) {
        parent::__construct($mensaje);
    }
}
