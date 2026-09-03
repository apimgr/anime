package server

import (
	"embed"
	"html/template"
	"net/http"
)

// Embed static files and templates
//go:embed static/*
var staticFS embed.FS

//go:embed templates/*
var templateFS embed.FS

var templates *template.Template

// initTemplates initializes the HTML templates
func initTemplates() error {
	var err error
	templates, err = template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return err
	}
	return nil
}

// serveStatic sets up the static file server for CSS, JS, and images
func (s *Server) serveStatic() http.Handler {
	return http.FileServer(http.FS(staticFS))
}
