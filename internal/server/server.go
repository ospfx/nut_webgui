package server

import (
	"embed"
	"html/template"
	"net/http"
)

//go:embed ../../web/templates/*.html
var tmplFS embed.FS

//go:embed ../../web/static/*
var staticFS embed.FS

type Server struct {
	mux  *http.ServeMux
	tmpl *template.Template
}

func New() *Server {
	t := template.Must(template.ParseFS(tmplFS, "../../web/templates/*.html"))
	mux := http.NewServeMux()

	// 首页
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_ = t.ExecuteTemplate(w, "index.html", map[string]any{
			"Title": "NUT Web GUI (Go Rewrite)",
		})
	})

	// 健康检查
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// 静态资源
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	return &Server{mux: mux, tmpl: t}
}

func (s *Server) Router() http.Handler { return s.mux }
