<?php

use PHPUnit\Framework\TestCase;

/** Nivel HTTP: Aplicacion con el servicio real y el repositorio sobre SQLite. */
final class AplicacionTest extends TestCase
{
    private PDO $pdo;
    private bool $baseDeDatosArriba = true;
    private int $conexionesAbiertas = 0;

    protected function setUp(): void
    {
        $this->pdo = BaseDeDatosDePrueba::crear();
    }

    private function aplicacion(): Aplicacion
    {
        return new Aplicacion(
            function (): DepartamentoService {
                $this->conexionesAbiertas++;
                return new DepartamentoService(new DepartamentoRepository($this->pdo));
            },
            fn (): bool => $this->baseDeDatosArriba,
        );
    }

    private function sinBaseDeDatos(): Aplicacion
    {
        return new Aplicacion(
            fn () => throw new PDOException('SQLSTATE[HY000] [2002] Connection refused'),
            fn (): bool => false,
        );
    }

    private function atender(string $metodo, string $ruta, mixed $cuerpo = null): array
    {
        $texto = is_string($cuerpo) ? $cuerpo : ($cuerpo === null ? '' : json_encode($cuerpo));
        $respuesta = $this->aplicacion()->atender($metodo, $ruta, $texto);
        return [$respuesta, json_decode($respuesta->cuerpo, true)];
    }

    private function crearIt(): void
    {
        [$respuesta] = $this->atender('POST', '/departamentos', ['id' => 'IT', 'nombre' => 'Tecnología', 'descripcion' => 'TI']);
        $this->assertSame(201, $respuesta->status);
    }

    private function assertError(Respuesta $respuesta, array $cuerpo, int $status, string $mensaje, string $ruta): void
    {
        $this->assertSame($status, $respuesta->status);
        $this->assertSame('application/json; charset=utf-8', $respuesta->cabeceras['Content-Type']);
        $this->assertSame($status, $cuerpo['status']);
        $this->assertSame([400 => 'Bad Request', 404 => 'Not Found', 500 => 'Internal Server Error'][$status], $cuerpo['error']);
        $this->assertSame($mensaje, $cuerpo['message']);
        $this->assertSame($ruta, $cuerpo['path']);
        $this->assertMatchesRegularExpression('/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/', $cuerpo['timestamp']);
    }

    public function testPostCreaConLocation(): void
    {
        [$respuesta, $cuerpo] = $this->atender('POST', '/departamentos', ['id' => 'IT', 'nombre' => 'Tecnología']);

        $this->assertSame(201, $respuesta->status);
        $this->assertSame('/departamentos/IT', $respuesta->cabeceras['Location']);
        $this->assertSame('application/json; charset=utf-8', $respuesta->cabeceras['Content-Type']);
        $this->assertSame(['id' => 'IT', 'nombre' => 'Tecnología', 'descripcion' => null], $cuerpo);
    }

    public function testPostSinCamposResponde400ConErrores(): void
    {
        [$respuesta, $cuerpo] = $this->atender('POST', '/departamentos', '');

        $this->assertError($respuesta, $cuerpo, 400, 'Faltan los campos obligatorios: id, nombre', '/departamentos');
        $this->assertSame(['id', 'nombre'], array_column($cuerpo['errors'], 'field'));
    }

    public function testPostConJsonQueNoEsObjetoValidaLosCampos(): void
    {
        [$respuesta, $cuerpo] = $this->atender('POST', '/departamentos', '"texto"');

        $this->assertError($respuesta, $cuerpo, 400, 'Faltan los campos obligatorios: id, nombre', '/departamentos');
    }

    public function testPostConJsonInvalidoResponde400SinErrores(): void
    {
        [$respuesta, $cuerpo] = $this->atender('POST', '/departamentos', '{no es json');

        $this->assertError($respuesta, $cuerpo, 400, 'Cuerpo JSON inválido', '/departamentos');
        $this->assertArrayNotHasKey('errors', $cuerpo);
    }

    public function testPostDuplicadoResponde400(): void
    {
        $this->crearIt();

        [$respuesta, $cuerpo] = $this->atender('POST', '/departamentos', ['id' => 'IT', 'nombre' => 'Otro']);

        $this->assertError($respuesta, $cuerpo, 400, 'El departamento con id IT ya está registrado', '/departamentos');
        $this->assertSame([['field' => 'id', 'message' => 'Ya está registrado', 'rejectedValue' => 'IT']], $cuerpo['errors']);
    }

    public function testGetListaEnOrden(): void
    {
        $this->crearIt();

        [$respuesta, $cuerpo] = $this->atender('GET', '/departamentos');

        $this->assertSame(200, $respuesta->status);
        $this->assertSame([['id' => 'IT', 'nombre' => 'Tecnología', 'descripcion' => 'TI']], $cuerpo);
    }

    public function testGetUno(): void
    {
        $this->crearIt();

        [$respuesta, $cuerpo] = $this->atender('GET', '/departamentos/IT');

        $this->assertSame(200, $respuesta->status);
        $this->assertSame('Tecnología', $cuerpo['nombre']);
    }

    public function testGetInexistenteResponde404(): void
    {
        [$respuesta, $cuerpo] = $this->atender('GET', '/departamentos/XX');

        $this->assertError($respuesta, $cuerpo, 404, 'El departamento con id XX no existe', '/departamentos/XX');
    }

    public function testPutActualizaYPersiste(): void
    {
        $this->crearIt();

        [$respuesta, $cuerpo] = $this->atender('PUT', '/departamentos/IT', ['nombre' => 'Innovación']);

        $this->assertSame(200, $respuesta->status);
        $this->assertSame(['id' => 'IT', 'nombre' => 'Innovación', 'descripcion' => null], $cuerpo);
        $this->assertSame($cuerpo, $this->atender('GET', '/departamentos/IT')[1]);
    }

    public function testPutSinNombreResponde400(): void
    {
        $this->crearIt();

        [$respuesta, $cuerpo] = $this->atender('PUT', '/departamentos/IT', ['descripcion' => 'x']);

        $this->assertError($respuesta, $cuerpo, 400, 'Faltan los campos obligatorios: nombre', '/departamentos/IT');
        $this->assertSame([['field' => 'nombre', 'message' => 'Es un campo obligatorio']], $cuerpo['errors']);
    }

    public function testPutCambiandoElIdResponde400(): void
    {
        $this->crearIt();

        [$respuesta, $cuerpo] = $this->atender('PUT', '/departamentos/IT', ['id' => 'TI', 'nombre' => 'Innovación']);

        $this->assertError($respuesta, $cuerpo, 400, 'No se puede cambiar el id del departamento', '/departamentos/IT');
        $this->assertSame('id', $cuerpo['errors'][0]['field']);
    }

    public function testPutConJsonInvalidoResponde400(): void
    {
        $this->crearIt();

        [$respuesta, $cuerpo] = $this->atender('PUT', '/departamentos/IT', '{');

        $this->assertError($respuesta, $cuerpo, 400, 'Cuerpo JSON inválido', '/departamentos/IT');
    }

    public function testPutInexistenteResponde404(): void
    {
        [$respuesta, $cuerpo] = $this->atender('PUT', '/departamentos/XX', ['nombre' => 'Nada']);

        $this->assertError($respuesta, $cuerpo, 404, 'El departamento con id XX no existe', '/departamentos/XX');
    }

    public function testDeleteResponde204SinCuerpoYLuegoNoExiste(): void
    {
        $this->crearIt();

        $respuesta = $this->aplicacion()->atender('DELETE', '/departamentos/IT');

        $this->assertSame(204, $respuesta->status);
        $this->assertSame('', $respuesta->cuerpo);
        $this->assertSame(404, $this->atender('GET', '/departamentos/IT')[0]->status);
    }

    public function testDeleteInexistenteResponde404(): void
    {
        [$respuesta, $cuerpo] = $this->atender('DELETE', '/departamentos/XX');

        $this->assertError($respuesta, $cuerpo, 404, 'El departamento con id XX no existe', '/departamentos/XX');
    }

    /** @return array<string, array{string, string}> */
    public static function rutasNoSoportadas(): array
    {
        return [
            'otra ruta' => ['GET', '/no-existe'],
            'PATCH con id' => ['PATCH', '/departamentos/IT'],
            'DELETE de la colección' => ['DELETE', '/departamentos'],
            'POST con id' => ['POST', '/departamentos/IT'],
            'subruta' => ['GET', '/departamentos/IT/empleados'],
        ];
    }

    #[\PHPUnit\Framework\Attributes\DataProvider('rutasNoSoportadas')]
    public function testRutaNoSoportadaResponde404(string $metodo, string $ruta): void
    {
        [$respuesta, $cuerpo] = $this->atender($metodo, $ruta);

        $this->assertError($respuesta, $cuerpo, 404, 'Recurso no encontrado', $ruta);
    }

    public function testErrorDeBaseDeDatosEnUnaOperacionResponde500(): void
    {
        $this->pdo->exec('DROP TABLE departamentos');

        [$respuesta, $cuerpo] = $this->atender('GET', '/departamentos');

        $this->assertError($respuesta, $cuerpo, 500, 'Error interno del servidor', '/departamentos');
    }

    public function testSinConexionResponde500(): void
    {
        $respuesta = $this->sinBaseDeDatos()->atender('GET', '/departamentos');

        $this->assertError($respuesta, json_decode($respuesta->cuerpo, true), 500, 'No fue posible conectar con la base de datos', '/departamentos');
    }

    public function testDocumentacionYSaludNoAbrenConexion(): void
    {
        $aplicacion = $this->aplicacion();
        $aplicacion->atender('GET', '/departamentos/docs');
        $aplicacion->atender('GET', '/departamentos/openapi.json');
        $aplicacion->atender('GET', '/health');

        $this->assertSame(0, $this->conexionesAbiertas);
    }

    public function testDocsEsHtmlAunqueLaBaseDeDatosEsteCaida(): void
    {
        $respuesta = $this->sinBaseDeDatos()->atender('GET', '/departamentos/docs');

        $this->assertSame(200, $respuesta->status);
        $this->assertSame('text/html; charset=utf-8', $respuesta->cabeceras['Content-Type']);
        $this->assertStringContainsString("url: '/departamentos/openapi.json'", $respuesta->cuerpo);
    }

    public function testOpenApiEsJsonAunqueLaBaseDeDatosEsteCaida(): void
    {
        $respuesta = $this->sinBaseDeDatos()->atender('GET', '/departamentos/openapi.json');

        $this->assertSame(200, $respuesta->status);
        $this->assertSame('application/json; charset=utf-8', $respuesta->cabeceras['Content-Type']);
        $this->assertSame('3.0.3', json_decode($respuesta->cuerpo, true)['openapi']);
    }

    public function testSaludArriba(): void
    {
        [$respuesta, $cuerpo] = $this->atender('GET', '/health');

        $this->assertSame(200, $respuesta->status);
        $this->assertSame('UP', $cuerpo['status']);
        $this->assertSame('departamentos-service', $cuerpo['service']);
        $this->assertSame(['database' => 'UP'], $cuerpo['checks']);
        $this->assertSame(['app' => 'UP', 'db' => 'UP'], $cuerpo['components']);
        $this->assertMatchesRegularExpression('/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/', $cuerpo['timestamp']);
    }

    public function testSaludConBaseDeDatosCaidaResponde503(): void
    {
        $this->baseDeDatosArriba = false;

        [$respuesta, $cuerpo] = $this->atender('GET', '/health');

        $this->assertSame(503, $respuesta->status);
        $this->assertSame('DOWN', $cuerpo['status']);
        $this->assertSame(['database' => 'DOWN'], $cuerpo['checks']);
        $this->assertSame(['app' => 'UP', 'db' => 'DOWN'], $cuerpo['components']);
    }

    public function testIdCodificadoEnLaRutaSeDecodifica(): void
    {
        $this->atender('POST', '/departamentos', ['id' => 'RRHH 1', 'nombre' => 'Personas']);

        [$respuesta, $cuerpo] = $this->atender('GET', '/departamentos/RRHH%201');
        $this->assertSame(200, $respuesta->status);
        $this->assertSame('RRHH 1', $cuerpo['id']);

        [$respuesta] = $this->atender('PUT', '/departamentos/RRHH%201', ['id' => 'RRHH 1', 'nombre' => 'Talento']);
        $this->assertSame(200, $respuesta->status);
    }

    public function testNombreMuyLargoResponde400YNo500(): void
    {
        [$respuesta, $cuerpo] = $this->atender('POST', '/departamentos', ['id' => 'IT', 'nombre' => str_repeat('n', 101)]);

        $this->assertSame(400, $respuesta->status);
        $this->assertSame('nombre', $cuerpo['errors'][0]['field']);
    }
}
