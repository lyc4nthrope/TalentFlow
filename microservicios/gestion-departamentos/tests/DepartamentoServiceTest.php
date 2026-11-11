<?php

use PHPUnit\Framework\TestCase;

final class DepartamentoServiceTest extends TestCase
{
    private RepositorioEnMemoria $repositorio;
    private DepartamentoService $servicio;

    protected function setUp(): void
    {
        $this->repositorio = new RepositorioEnMemoria();
        $this->servicio = new DepartamentoService($this->repositorio);
    }

    private function fallaCon(callable $accion): ErrorDeNegocio
    {
        try {
            $accion();
        } catch (ErrorDeNegocio $e) {
            return $e;
        }
        $this->fail('Se esperaba un ErrorDeNegocio');
    }

    public function testCrearGuardaYDevuelveElDepartamento(): void
    {
        $creado = $this->servicio->crear(['id' => 'IT', 'nombre' => 'Tecnología', 'descripcion' => 'TI']);

        $this->assertSame(['id' => 'IT', 'nombre' => 'Tecnología', 'descripcion' => 'TI'], $creado);
        $this->assertSame($creado, $this->repositorio->buscarPorId('IT'));
    }

    public function testCrearSinCamposObligatoriosLosEnumera(): void
    {
        $error = $this->fallaCon(fn () => $this->servicio->crear(['descripcion' => 'x']));

        $this->assertSame(400, $error->status);
        $this->assertSame('Faltan los campos obligatorios: id, nombre', $error->getMessage());
        $this->assertSame([
            ['field' => 'id', 'message' => 'Es un campo obligatorio'],
            ['field' => 'nombre', 'message' => 'Es un campo obligatorio'],
        ], $error->errores);
        $this->assertSame([], $this->repositorio->filas);
    }

    public function testCrearConNombreVacioFalta(): void
    {
        $error = $this->fallaCon(fn () => $this->servicio->crear(['id' => 'IT', 'nombre' => '']));

        $this->assertSame('Faltan los campos obligatorios: nombre', $error->getMessage());
    }

    public function testCrearConIdDuplicadoResponde400(): void
    {
        $this->servicio->crear(['id' => 'IT', 'nombre' => 'Tecnología']);

        $error = $this->fallaCon(fn () => $this->servicio->crear(['id' => 'IT', 'nombre' => 'Otro']));

        $this->assertSame(400, $error->status);
        $this->assertSame('El departamento con id IT ya está registrado', $error->getMessage());
        $this->assertSame([['field' => 'id', 'message' => 'Ya está registrado', 'rejectedValue' => 'IT']], $error->errores);
        $this->assertSame('Tecnología', $this->repositorio->buscarPorId('IT')['nombre']);
    }

    public function testCrearEnCarreraConSqlstate23000EsDuplicado(): void
    {
        $this->repositorio->falloAlGuardar = RepositorioEnMemoria::errorPdo('23000');

        $error = $this->fallaCon(fn () => $this->servicio->crear(['id' => 'IT', 'nombre' => 'Tecnología']));

        $this->assertSame(400, $error->status);
        $this->assertSame('El departamento con id IT ya está registrado', $error->getMessage());
    }

    public function testCrearConOtroErrorDeBaseDeDatosLoPropaga(): void
    {
        $this->repositorio->falloAlGuardar = RepositorioEnMemoria::errorPdo('HY000');

        $this->expectException(PDOException::class);
        $this->servicio->crear(['id' => 'IT', 'nombre' => 'Tecnología']);
    }

    public function testListarDevuelveTodos(): void
    {
        $this->servicio->crear(['id' => 'IT', 'nombre' => 'Tecnología']);
        $this->servicio->crear(['id' => 'RH', 'nombre' => 'Recursos Humanos']);

        $this->assertSame(['IT', 'RH'], array_column($this->servicio->listar(), 'id'));
    }

    public function testObtenerExistente(): void
    {
        $this->servicio->crear(['id' => 'IT', 'nombre' => 'Tecnología']);

        $this->assertSame('Tecnología', $this->servicio->obtener('IT')['nombre']);
    }

    public function testObtenerInexistenteResponde404(): void
    {
        $error = $this->fallaCon(fn () => $this->servicio->obtener('XX'));

        $this->assertSame(404, $error->status);
        $this->assertSame('El departamento con id XX no existe', $error->getMessage());
        $this->assertSame([], $error->errores);
    }

    public function testActualizarReemplazaNombreYDescripcion(): void
    {
        $this->servicio->crear(['id' => 'IT', 'nombre' => 'Tecnología', 'descripcion' => 'TI']);

        $actualizado = $this->servicio->actualizar('IT', ['nombre' => 'Innovación', 'descripcion' => 'I+D']);

        $this->assertSame(['id' => 'IT', 'nombre' => 'Innovación', 'descripcion' => 'I+D'], $actualizado);
        $this->assertSame($actualizado, $this->repositorio->buscarPorId('IT'));
    }

    public function testActualizarSinDescripcionLaDejaEnNull(): void
    {
        $this->servicio->crear(['id' => 'IT', 'nombre' => 'Tecnología', 'descripcion' => 'TI']);

        $this->assertNull($this->servicio->actualizar('IT', ['nombre' => 'Innovación'])['descripcion']);
    }

    public function testActualizarConElMismoIdEnElCuerpoEsValido(): void
    {
        $this->servicio->crear(['id' => 'IT', 'nombre' => 'Tecnología']);

        $this->assertSame('Innovación', $this->servicio->actualizar('IT', ['id' => 'IT', 'nombre' => 'Innovación'])['nombre']);
    }

    public function testActualizarSinNombreResponde400(): void
    {
        $this->servicio->crear(['id' => 'IT', 'nombre' => 'Tecnología']);

        $error = $this->fallaCon(fn () => $this->servicio->actualizar('IT', ['descripcion' => 'x']));

        $this->assertSame(400, $error->status);
        $this->assertSame('Faltan los campos obligatorios: nombre', $error->getMessage());
        $this->assertSame([['field' => 'nombre', 'message' => 'Es un campo obligatorio']], $error->errores);
        $this->assertSame('Tecnología', $this->repositorio->buscarPorId('IT')['nombre']);
    }

    public function testActualizarNoPermiteCambiarElId(): void
    {
        $this->servicio->crear(['id' => 'IT', 'nombre' => 'Tecnología']);

        $error = $this->fallaCon(fn () => $this->servicio->actualizar('IT', ['id' => 'TI', 'nombre' => 'Innovación']));

        $this->assertSame(400, $error->status);
        $this->assertSame('No se puede cambiar el id del departamento', $error->getMessage());
        $this->assertSame([['field' => 'id', 'message' => 'No se puede cambiar el id', 'rejectedValue' => 'TI']], $error->errores);
        $this->assertSame('Tecnología', $this->repositorio->buscarPorId('IT')['nombre']);
        $this->assertNull($this->repositorio->buscarPorId('TI'));
    }

    public function testActualizarInexistenteResponde404(): void
    {
        $error = $this->fallaCon(fn () => $this->servicio->actualizar('XX', ['nombre' => 'Nada']));

        $this->assertSame(404, $error->status);
        $this->assertSame('El departamento con id XX no existe', $error->getMessage());
        $this->assertSame([], $this->repositorio->filas);
    }

    public function testEliminarExistente(): void
    {
        $this->servicio->crear(['id' => 'IT', 'nombre' => 'Tecnología']);
        $this->servicio->crear(['id' => 'RH', 'nombre' => 'Recursos Humanos']);

        $this->servicio->eliminar('IT');

        $this->assertNull($this->repositorio->buscarPorId('IT'));
        $this->assertNotNull($this->repositorio->buscarPorId('RH'));
    }

    public function testEliminarInexistenteResponde404(): void
    {
        $error = $this->fallaCon(fn () => $this->servicio->eliminar('XX'));

        $this->assertSame(404, $error->status);
        $this->assertSame('El departamento con id XX no existe', $error->getMessage());
    }

    public function testCrearConCamposMasLargosQueLaColumnaResponde400(): void
    {
        $error = $this->fallaCon(fn () => $this->servicio->crear([
            'id' => str_repeat('I', 21),
            'nombre' => str_repeat('n', 101),
            'descripcion' => str_repeat('d', 256),
        ]));

        $this->assertSame(400, $error->status);
        $this->assertSame('Campos con formato inválido: id, nombre, descripcion', $error->getMessage());
        $this->assertSame([
            ['field' => 'id', 'message' => 'Máximo 20 caracteres'],
            ['field' => 'nombre', 'message' => 'Máximo 100 caracteres'],
            ['field' => 'descripcion', 'message' => 'Máximo 255 caracteres'],
        ], $error->errores);
        $this->assertSame([], $this->repositorio->filas);
    }

    public function testCrearEnElLimiteDeLaColumnaEsValido(): void
    {
        // Caracteres, no bytes: utf8mb4 en MySQL cuenta "ñ" como uno.
        $creado = $this->servicio->crear([
            'id' => str_repeat('I', 20),
            'nombre' => str_repeat('ñ', 100),
            'descripcion' => str_repeat('d', 255),
        ]);

        $this->assertSame(str_repeat('ñ', 100), $creado['nombre']);
    }

    public function testActualizarConNombreQueNoEsTextoResponde400(): void
    {
        $this->servicio->crear(['id' => 'IT', 'nombre' => 'Tecnología']);

        $error = $this->fallaCon(fn () => $this->servicio->actualizar('IT', ['nombre' => ['x'], 'descripcion' => ['y']]));

        $this->assertSame(400, $error->status);
        $this->assertSame([
            ['field' => 'nombre', 'message' => 'Debe ser texto'],
            ['field' => 'descripcion', 'message' => 'Debe ser texto'],
        ], $error->errores);
        $this->assertSame('Tecnología', $this->repositorio->buscarPorId('IT')['nombre']);
    }

    public function testActualizarConDescripcionMuyLargaResponde400(): void
    {
        $this->servicio->crear(['id' => 'IT', 'nombre' => 'Tecnología']);

        $error = $this->fallaCon(fn () => $this->servicio->actualizar('IT', ['nombre' => 'TI', 'descripcion' => str_repeat('d', 256)]));

        $this->assertSame([['field' => 'descripcion', 'message' => 'Máximo 255 caracteres']], $error->errores);
    }
}
