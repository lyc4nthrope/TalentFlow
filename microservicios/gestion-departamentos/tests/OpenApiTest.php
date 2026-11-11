<?php

use PHPUnit\Framework\TestCase;

final class OpenApiTest extends TestCase
{
    public function testDocumentaTodasLasOperaciones(): void
    {
        $paths = obtenerEspecificacionOpenApi()['paths'];

        $this->assertSame(['post', 'get'], array_keys($paths['/departamentos']));
        $this->assertSame(['get', 'put', 'delete'], array_keys($paths['/departamentos/{id}']));
    }

    public function testPutDocumentaCuerpoYRespuestas(): void
    {
        $put = obtenerEspecificacionOpenApi()['paths']['/departamentos/{id}']['put'];

        $this->assertSame(['nombre'], $put['requestBody']['content']['application/json']['schema']['required']);
        $this->assertSame(['200', '400', '404'], array_map('strval', array_keys($put['responses'])));
        $this->assertSame('id', $put['parameters'][0]['name']);
    }

    public function testDeleteDocumenta204Y404(): void
    {
        $delete = obtenerEspecificacionOpenApi()['paths']['/departamentos/{id}']['delete'];

        $this->assertSame(['204', '404'], array_map('strval', array_keys($delete['responses'])));
        $this->assertArrayNotHasKey('content', $delete['responses']['204']);
    }

    public function testSeguridadSeSerializaComoListaVacia(): void
    {
        $json = json_encode(obtenerEspecificacionOpenApi());

        $this->assertStringContainsString('"security":[{"bearerAuth":[]}]', $json);
    }

    public function testHtmlDeSwaggerApuntaALaEspecificacion(): void
    {
        $this->assertStringContainsString("url: '/departamentos/openapi.json'", obtenerHtmlSwagger());
    }
}
