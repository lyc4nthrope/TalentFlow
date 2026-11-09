<?php

// Sin Composer en producción: carga explícita de las clases (la usan index.php y las pruebas).
require_once __DIR__ . '/Database.php';
require_once __DIR__ . '/ErrorDeNegocio.php';
require_once __DIR__ . '/RepositorioDepartamentos.php';
require_once __DIR__ . '/DepartamentoRepository.php';
require_once __DIR__ . '/DepartamentoService.php';
require_once __DIR__ . '/Respuesta.php';
require_once __DIR__ . '/Aplicacion.php';
require_once __DIR__ . '/OpenApi.php';
require_once __DIR__ . '/Metricas.php';
require_once __DIR__ . '/Trazas.php';
