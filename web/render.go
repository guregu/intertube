package web

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
)

func renderText(w http.ResponseWriter, text string, code int) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	setCacheHeaders(w)
	w.WriteHeader(code)
	_, err := io.WriteString(w, text)
	if err != nil {
		panic(err)
	}
}

func renderJSON(w http.ResponseWriter, data any, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	setCacheHeaders(w)
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		panic(err)
	}
}

func renderTemplate(ctx context.Context, w http.ResponseWriter, tmpl string, data any, code int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	setCacheHeaders(w)
	w.WriteHeader(code)
	if err := getTemplate(ctx, tmpl).Execute(w, data); err != nil {
		panic(err)
	}
}

func renderXML(w http.ResponseWriter, data any, code int) {
	w.Header().Set("Content-Type", "text/xml")
	setCacheHeaders(w)
	w.WriteHeader(code)

	if _, err := io.WriteString(w, xml.Header); err != nil {
		panic(err)
	}
	if err := xml.NewEncoder(w).Encode(data); err != nil {
		panic(err)
	}
}

func setCacheHeaders(w http.ResponseWriter) {
	if w.Header().Get("Cache-Control") == "" {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	}
}
