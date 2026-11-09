<?php

/** SQLite en memoria con el esquema real de init.sql (mismo SQL que MySQL usa al iniciar). */
final class BaseDeDatosDePrueba
{
    public static function crear(): PDO
    {
        $pdo = new PDO('sqlite::memory:', null, null, [
            PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
            PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
        ]);
        $pdo->exec(file_get_contents(__DIR__ . '/../../init.sql'));
        return $pdo;
    }
}
