package api

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.json
var especificacionOpenAPI []byte

func servirOpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(especificacionOpenAPI)
}

func servirSwaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(paginaSwagger))
}

// persistAuthorization: el token pegado en "Authorize" sobrevive a recargar la página.
const paginaSwagger = `<!doctype html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <title>Auth - Swagger UI</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.11.0/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.11.0/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({ url: "/auth/openapi.json", dom_id: "#swagger-ui", persistAuthorization: true });
  </script>
</body>
</html>`
