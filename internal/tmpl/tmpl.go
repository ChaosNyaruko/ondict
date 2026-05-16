// Package tmpl exposes the embedded HTML templates so both the main server
// and the mobile entry point can share the same template files.
package tmpl

import (
	"embed"
	"html/template"

	log "github.com/sirupsen/logrus"
)

//go:embed templates
var FS embed.FS

// Must parses all templates from the embedded filesystem and panics on error.
func Must() *template.Template {
	tpl, err := template.ParseFS(FS, "templates/*.html")
	if err != nil {
		log.Fatalf("tmpl: parse templates: %v", err)
	}
	return tpl
}
