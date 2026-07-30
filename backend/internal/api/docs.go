package api

import (
	_ "embed"
	"net/http"

	"github.com/labstack/echo/v5"
)

//go:embed openapi.yaml
var openAPISpec []byte

const swaggerHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>EDMS API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: "/openapi.yaml",
      dom_id: "#swagger-ui"
    });
  </script>
</body>
</html>`

func (s *Server) openAPI(c *echo.Context) error {
	return c.Blob(http.StatusOK, "application/yaml", openAPISpec)
}

func (s *Server) swagger(c *echo.Context) error {
	return c.HTML(http.StatusOK, swaggerHTML)
}
