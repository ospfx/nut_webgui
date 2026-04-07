package server

import (
	"fmt"
	"html/template"
	"io/fs"
	"strings"

	"github.com/ospfx/nut_webgui/web"
)

// templateFuncs returns the template function map.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"upper":       strings.ToUpper,
		"lower":       strings.ToLower,
		"replace":     strings.ReplaceAll,
		"hasPrefix":   strings.HasPrefix,
		"statusClass": upsStatusClass,
	}
}

// upsStatusClass returns a CSS class suffix for a UPS status string.
func upsStatusClass(status string) string {
	switch {
	case strings.Contains(status, "OL") && !strings.Contains(status, "OB"):
		return "status-online"
	case strings.Contains(status, "OB"):
		return "status-onbattery"
	case strings.Contains(status, "LB"):
		return "status-lowbattery"
	case strings.Contains(status, "FSD"):
		return "status-fsd"
	default:
		return "status-unknown"
	}
}

// parseTemplates parses all HTML templates.
// Each page template is combined with layout.html so they share the "layout" define.
func parseTemplates() (map[string]*template.Template, error) {
	layoutData, err := fs.ReadFile(web.Templates, "templates/layout.html")
	if err != nil {
		return nil, fmt.Errorf("read layout: %w", err)
	}

	entries, err := fs.ReadDir(web.Templates, "templates")
	if err != nil {
		return nil, fmt.Errorf("read templates dir: %w", err)
	}

	templates := make(map[string]*template.Template)
	for _, e := range entries {
		if e.IsDir() || e.Name() == "layout.html" {
			continue
		}
		pageData, err := fs.ReadFile(web.Templates, "templates/"+e.Name())
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", e.Name(), err)
		}
		// Parse layout first, then page (which overrides the "content" block)
		t, err := template.New("layout").Funcs(templateFuncs()).Parse(string(layoutData))
		if err != nil {
			return nil, fmt.Errorf("parse layout for %s: %w", e.Name(), err)
		}
		if _, err := t.New("content").Parse(string(pageData)); err != nil {
			return nil, fmt.Errorf("parse page %s: %w", e.Name(), err)
		}
		templates[e.Name()] = t
	}

	return templates, nil
}
