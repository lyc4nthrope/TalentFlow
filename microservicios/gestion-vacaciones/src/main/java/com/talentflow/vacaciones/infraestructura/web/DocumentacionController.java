package com.talentflow.vacaciones.infraestructura.web;

import io.swagger.v3.oas.annotations.Hidden;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * Swagger UI bajo el prefijo del servicio (/vacaciones/docs), el único que enruta el Gateway.
 * springdoc genera la especificación en /vacaciones/openapi.json; su UI propia vive en
 * /swagger-ui/**, fuera del prefijo, por eso se sirve esta página.
 */
@RestController
@Hidden
public class DocumentacionController {

    private static final String PAGINA = """
            <!doctype html>
            <html lang="es">
            <head>
              <meta charset="utf-8">
              <title>Vacaciones - Swagger UI</title>
              <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
            </head>
            <body>
              <div id="swagger-ui"></div>
              <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
              <script>
                window.ui = SwaggerUIBundle({ url: "/vacaciones/openapi.json", dom_id: "#swagger-ui" });
              </script>
            </body>
            </html>
            """;

    @GetMapping(value = "/vacaciones/docs", produces = MediaType.TEXT_HTML_VALUE)
    public String swaggerUi() {
        return PAGINA;
    }
}
