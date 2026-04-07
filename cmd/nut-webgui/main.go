package main

import (
	"log"
	"net/http"

	"github.com/ospfx/nut_webgui/internal/server"
)

func main() {
	s := server.New()
	log.Println("nut_webgui listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", s.Router()))
}
