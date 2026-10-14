// Package web contains the HTTP handlers, built only with the net/http standard library.
package web

import (
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/Akshayjolly/url-shortener-go/internal/shortener"
)

//go:embed templates/*.html
var templateFiles embed.FS

//go:embed static/*
var staticFiles embed.FS

// Server holds what the handlers need.
type Server struct {
	service *shortener.Service
	baseURL string // e.g. "http://localhost:8080", used to show the full short link
	pages   *template.Template
}

// NewServer parses the HTML templates and returns a Server.
func NewServer(service *shortener.Service, baseURL string) *Server {
	pages := template.Must(template.ParseFS(templateFiles, "templates/*.html"))
	return &Server{service: service, baseURL: strings.TrimRight(baseURL, "/"), pages: pages}
}

// Routes registers every endpoint. Go 1.22+ patterns allow the method and {code} wildcards.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("POST /shorten", s.shortenForm)
	mux.HandleFunc("GET /stats/{code}", s.stats)
	mux.HandleFunc("POST /api/links", s.apiCreate)
	mux.HandleFunc("GET /api/links/{code}", s.apiGet)
	mux.Handle("GET /static/", http.FileServerFS(staticFiles))
	mux.HandleFunc("GET /{code}", s.redirect)
	return logRequests(mux)
}

// pageData is what the templates receive.
type pageData struct {
	BaseURL string
	Created *shortener.Link
	Link    *shortener.Link
	Recent  []shortener.Link
	Error   string
	URL     string
	Code    string
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	s.renderHome(w, r, http.StatusOK, pageData{})
}

// shortenForm handles the HTML form on the home page.
func (s *Server) shortenForm(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	data := pageData{URL: r.FormValue("url"), Code: r.FormValue("code")}
	link, err := s.service.Shorten(r.Context(), data.URL, data.Code)
	if err != nil {
		data.Error = userMessage(err)
		s.renderHome(w, r, statusFor(err), data)
		return
	}
	data.Created = &link
	data.URL, data.Code = "", ""
	s.renderHome(w, r, http.StatusCreated, data)
}

func (s *Server) renderHome(w http.ResponseWriter, r *http.Request, status int, data pageData) {
	data.BaseURL = s.baseURL
	recent, err := s.service.Recent(r.Context(), 10)
	if err != nil {
		log.Printf("recent links: %v", err)
	}
	data.Recent = recent
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := s.pages.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("template: %v", err)
	}
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	link, err := s.service.Lookup(r.Context(), r.PathValue("code"))
	if err != nil {
		http.Error(w, userMessage(err), statusFor(err))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.pages.ExecuteTemplate(w, "stats.html", pageData{BaseURL: s.baseURL, Link: &link}); err != nil {
		log.Printf("template: %v", err)
	}
}

// redirect sends the visitor to the original URL and counts the click.
func (s *Server) redirect(w http.ResponseWriter, r *http.Request) {
	link, err := s.service.Resolve(r.Context(), r.PathValue("code"))
	if err != nil {
		http.Error(w, userMessage(err), statusFor(err))
		return
	}
	// 302 (not 301) so browsers don't cache it and every click reaches us to be counted.
	http.Redirect(w, r, link.TargetURL, http.StatusFound)
}

type createRequest struct {
	URL  string `json:"url"`
	Code string `json:"code,omitempty"`
}

type createResponse struct {
	shortener.Link
	ShortURL string `json:"shortUrl"`
}

// apiCreate: POST /api/links {"url": "...", "code": "optional"}
func (s *Server) apiCreate(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	link, err := s.service.Shorten(r.Context(), req.URL, req.Code)
	if err != nil {
		writeJSON(w, statusFor(err), map[string]string{"error": userMessage(err)})
		return
	}
	writeJSON(w, http.StatusCreated, createResponse{Link: link, ShortURL: s.baseURL + "/" + link.Code})
}

// apiGet: GET /api/links/{code} - details including click count
func (s *Server) apiGet(w http.ResponseWriter, r *http.Request) {
	link, err := s.service.Lookup(r.Context(), r.PathValue("code"))
	if err != nil {
		writeJSON(w, statusFor(err), map[string]string{"error": userMessage(err)})
		return
	}
	writeJSON(w, http.StatusOK, createResponse{Link: link, ShortURL: s.baseURL + "/" + link.Code})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// statusFor maps service errors to HTTP status codes.
func statusFor(err error) int {
	switch {
	case errors.Is(err, shortener.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, shortener.ErrCodeTaken):
		return http.StatusConflict
	case errors.Is(err, shortener.ErrInvalidURL), errors.Is(err, shortener.ErrInvalidCode):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// userMessage hides internal error details (like SQL errors) from users.
func userMessage(err error) string {
	if statusFor(err) == http.StatusInternalServerError {
		log.Printf("internal error: %v", err)
		return "something went wrong, please try again"
	}
	return err.Error()
}

// logRequests is a tiny middleware that logs every request.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
