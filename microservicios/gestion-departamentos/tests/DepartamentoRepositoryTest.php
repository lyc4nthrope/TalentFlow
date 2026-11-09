<?php

use PHPUnit\Framework\TestCase;

/** Integración: SQL real del repositorio contra SQLite con el esquema de init.sql. */
final class DepartamentoRepositoryTest extends TestCase
{
    private PDO $pdo;
    private DepartamentoRepository $repositorio;

    protected function setUp(): void
    {
        $this->pdo = BaseDeDatosDePrueba::crear();
        $this->repositorio = new DepartamentoRepository($this->pdo);
    }

    public function testGuardarYBuscarPorId(): void
    {
        $guardado = $this->repositorio->guardar(['id' => 'IT', 'nombre' => 'Tecnología', 'descripcion' => 'TI', 'extra' => 'se ignora']);

        $this->assertSame(['id' => 'IT', 'nombre' => 'Tecnología', 'descripcion' => 'TI'], $guardado);
        $this->assertSame($guardado, $this->repositorio->buscarPorId('IT'));
    }

    public function testGuardarSinDescripcionLaDejaEnNull(): void
    {
        $this->assertNull($this->repositorio->guardar(['id' => 'IT', 'nombre' => 'Tecnología'])['descripcion']);
    }

    public function testBuscarInexistenteDevuelveNull(): void
    {
        $this->assertNull($this->repositorio->buscarPorId('XX'));
    }

    public function testGuardarIdDuplicadoLanzaSqlstate23000(): void
    {
        $this->repositorio->guardar(['id' => 'IT', 'nombre' => 'Tecnología']);

        try {
            $this->repositorio->guardar(['id' => 'IT', 'nombre' => 'Otro']);
            $this->fail('Se esperaba la violación de PRIMARY KEY');
        } catch (PDOException $e) {
            $this->assertSame('23000', $e->getCode());
        }
    }

    public function testListarOrdenaPorFechaDeCreacion(): void
    {
        // Fechas explícitas: CURRENT_TIMESTAMP tiene resolución de segundos.
        $insertar = $this->pdo->prepare('INSERT INTO departamentos (id, nombre, creado_en) VALUES (?, ?, ?)');
        $insertar->execute(['B', 'Segundo', '2026-01-02 00:00:00']);
        $insertar->execute(['C', 'Tercero', '2026-01-03 00:00:00']);
        $insertar->execute(['A', 'Primero', '2026-01-01 00:00:00']);

        $lista = $this->repositorio->listar();

        $this->assertSame(['A', 'B', 'C'], array_column($lista, 'id'));
        $this->assertSame(['id', 'nombre', 'descripcion'], array_keys($lista[0]));
    }

    public function testListarVacio(): void
    {
        $this->assertSame([], $this->repositorio->listar());
    }

    public function testActualizarCambiaSoloEseDepartamento(): void
    {
        $this->repositorio->guardar(['id' => 'IT', 'nombre' => 'Tecnología', 'descripcion' => 'TI']);
        $this->repositorio->guardar(['id' => 'RH', 'nombre' => 'Recursos Humanos', 'descripcion' => 'Personas']);

        $actualizado = $this->repositorio->actualizar('IT', ['nombre' => 'Innovación', 'descripcion' => null]);

        $this->assertSame(['id' => 'IT', 'nombre' => 'Innovación', 'descripcion' => null], $actualizado);
        $this->assertSame(['id' => 'RH', 'nombre' => 'Recursos Humanos', 'descripcion' => 'Personas'], $this->repositorio->buscarPorId('RH'));
    }

    public function testActualizarConLosMismosValoresDevuelveElDepartamento(): void
    {
        $this->repositorio->guardar(['id' => 'IT', 'nombre' => 'Tecnología']);

        $this->assertSame('Tecnología', $this->repositorio->actualizar('IT', ['nombre' => 'Tecnología'])['nombre']);
    }

    public function testActualizarInexistenteDevuelveNull(): void
    {
        $this->assertNull($this->repositorio->actualizar('XX', ['nombre' => 'Nada']));
        $this->assertSame([], $this->repositorio->listar());
    }

    public function testEliminarBorraSoloEseDepartamento(): void
    {
        $this->repositorio->guardar(['id' => 'IT', 'nombre' => 'Tecnología']);
        $this->repositorio->guardar(['id' => 'RH', 'nombre' => 'Recursos Humanos']);

        $this->assertTrue($this->repositorio->eliminar('IT'));

        $this->assertNull($this->repositorio->buscarPorId('IT'));
        $this->assertSame(['RH'], array_column($this->repositorio->listar(), 'id'));
    }

    public function testEliminarInexistenteDevuelveFalse(): void
    {
        $this->repositorio->guardar(['id' => 'IT', 'nombre' => 'Tecnología']);

        $this->assertFalse($this->repositorio->eliminar('XX'));
        $this->assertCount(1, $this->repositorio->listar());
    }

    public function testListarConLaMismaFechaDesempataPorId(): void
    {
        $insertar = $this->pdo->prepare('INSERT INTO departamentos (id, nombre, creado_en) VALUES (?, ?, ?)');
        foreach (['C', 'A', 'B'] as $id) {
            $insertar->execute([$id, "Depto $id", '2026-01-01 00:00:00']);
        }

        $this->assertSame(['A', 'B', 'C'], array_column($this->repositorio->listar(), 'id'));
    }
}
