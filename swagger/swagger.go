package swagger

import (
	"fmt"
	"net/http"
)

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>%s - Swagger UI</title>
  <link rel="stylesheet" type="text/css" href="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.11.0/swagger-ui.css" />
  <style>
    html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
    *, *:before, *:after { box-sizing: inherit; }
    body { margin:0; background: #fafafa; }
    .swagger-ui .topbar { display: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.11.0/swagger-ui-bundle.js"></script>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.11.0/swagger-ui-standalone-preset.js"></script>
  <script>
  window.onload = function() {
    window.ui = SwaggerUIBundle({
      url: "%s",
      dom_id: '#swagger-ui',
      deepLinking: true,
      persistAuthorization: true,
      displayRequestDuration: true,
      filter: true,
      presets: [
        SwaggerUIBundle.presets.apis,
        SwaggerUIStandalonePreset
      ],
      plugins: [
        SwaggerUIBundle.plugins.DownloadUrl
      ],
      layout: "StandaloneLayout"
    });
  };
  </script>
</body>
</html>`

func RegisterSwagger(mux *http.ServeMux, env, serviceTitle, specJSON string) {
	if env != "dev" && env != "development" && env != "local" {
		return // In staging and production, swagger is strictly not registered
	}

	specPath := "/api/v1/docs/doc.json"
	serveJSON := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(specJSON))
	}

	serveHTML := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		html := fmt.Sprintf(swaggerUIHTML, serviceTitle, specPath)
		_, _ = w.Write([]byte(html))
	}

	// Standard API Docs endpoints
	mux.HandleFunc("GET /api/v1/docs/doc.json", serveJSON)
	mux.HandleFunc("GET /api/v1/docs/", serveHTML)
	mux.HandleFunc("GET /api/v1/docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/v1/docs/", http.StatusMovedPermanently)
	})

	mux.HandleFunc("GET /docs/doc.json", serveJSON)
	mux.HandleFunc("GET /docs/", serveHTML)
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/v1/docs/", http.StatusMovedPermanently)
	})

	// Legacy /swagger/ compatibility
	mux.HandleFunc("GET /swagger/doc.json", serveJSON)
	mux.HandleFunc("GET /swagger/", serveHTML)
	mux.HandleFunc("GET /swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/v1/docs/", http.StatusMovedPermanently)
	})
}
