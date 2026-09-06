package server

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/TheOutdoorProgrammer/manifests.io/internal/schema"
)

type Config struct {
	WebDir    string
	RenderDir string
	PublicDir string
	SiteURL   string
}

type Catalog interface {
	Page(schema.Query) (schema.Page, error)
	Products() []schema.Product
}

type Server struct {
	catalog Catalog
	config  Config
	shell   []byte
}

func New(catalog Catalog, config Config) (*Server, error) {
	shell, err := os.ReadFile(filepath.Join(config.WebDir, "index.html"))
	if err != nil {
		return nil, fmt.Errorf("read frontend build: %w", err)
	}
	for _, marker := range []string{"<!--app-html-->", "<!--page-data-->", "<!--page-head-->"} {
		if !bytes.Contains(shell, []byte(marker)) {
			return nil, fmt.Errorf("frontend template is missing %s", marker)
		}
	}
	if config.SiteURL == "" {
		config.SiteURL = "https://www.manifests.io"
	}
	site, err := url.Parse(config.SiteURL)
	if err != nil || site.Host == "" || (site.Scheme != "https" && site.Scheme != "http") || site.User != nil || site.RawQuery != "" || site.Fragment != "" || (site.Path != "" && site.Path != "/") {
		return nil, errors.New("SITE_URL must be an HTTP(S) origin")
	}
	config.SiteURL = strings.TrimRight(config.SiteURL, "/")
	return &Server{catalog: catalog, config: config, shell: shell}, nil
}

func RenderFilename(canonical string) string {
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:]) + ".html.gz"
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self' https://faro-collector-prod-us-east-3.grafana.net https://g.theoutdoorprogrammer.com; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if len(r.URL.RequestURI()) > 8192 {
		http.Error(w, "URL is too long", http.StatusRequestURITooLong)
		return
	}
	switch r.URL.Path {
	case "/healthz", "/readyz":
		r.Pattern = r.URL.Path
		writeJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
		return
	case "/":
		r.Pattern = "/"
		http.Redirect(w, r, "/kubernetes/1.34", http.StatusTemporaryRedirect)
		return
	case "/api/catalog":
		r.Pattern = "/api/catalog"
		writeJSON(w, r, http.StatusOK, s.catalog.Products())
		return
	}
	if strings.HasPrefix(r.URL.Path, "/assets/") {
		r.Pattern = "/assets/{file}"
		s.serveFile(w, r, s.config.WebDir, true)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") && strings.Count(r.URL.Path, "/") == 1 && strings.Contains(filepath.Base(r.URL.Path), ".") {
		r.Pattern = "/{asset}"
		s.serveFile(w, r, s.config.PublicDir, false)
		return
	}
	query, err := parseQuery(r)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, schema.ErrNotFound) {
			status = http.StatusNotFound
		}
		s.failure(w, r, status, "This documentation URL is invalid.")
		return
	}
	if r.URL.Path == "/api/page" {
		r.Pattern = "/api/page"
	} else if query.Resource != "" {
		r.Pattern = "/{item}/{version}/{resource}"
	} else {
		r.Pattern = "/{item}/{version}"
	}
	page, err := s.catalog.Page(query)
	if err != nil {
		status, message := http.StatusInternalServerError, "The documentation could not be loaded."
		if errors.Is(err, schema.ErrNotFound) {
			status, message = http.StatusNotFound, "This resource or version was not found."
		} else if errors.Is(err, schema.ErrBadQuery) {
			status, message = http.StatusBadRequest, "This documentation URL is invalid."
		}
		s.failure(w, r, status, message)
		return
	}
	if r.URL.Path == "/api/page" {
		writeJSON(w, r, http.StatusOK, page)
		return
	}
	s.servePage(w, r, http.StatusOK, page)
}

func parseQuery(r *http.Request) (schema.Query, error) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return schema.Query{}, err
	}
	for key, values := range values {
		if len(values) != 1 {
			return schema.Query{}, fmt.Errorf("duplicate parameter %s", key)
		}
	}
	q := schema.Query{Path: values.Get("path"), Pointer: values.Get("pointer"), Trail: values.Get("trail"), OneOf: values.Get("oneOf"), Key: values.Get("key")}
	if q.Path == "" {
		q.Path = values.Get("linked")
	}
	if r.URL.Path == "/api/page" {
		q.Item, q.Version, q.Resource = values.Get("item"), values.Get("version"), values.Get("resource")
	} else {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if len(parts) < 2 || len(parts) > 3 || strings.HasPrefix(r.URL.Path, "/api/") {
			return q, schema.ErrNotFound
		}
		q.Item, q.Version = parts[0], parts[1]
		if len(parts) == 3 {
			q.Resource = parts[2]
		}
	}
	if q.Item == "" || q.Version == "" || strings.ContainsAny(q.Item+q.Version+q.Resource, "/\\\x00") {
		return q, schema.ErrBadQuery
	}
	return q, nil
}

func (s *Server) serveFile(w http.ResponseWriter, r *http.Request, root string, immutable bool) {
	rel := strings.TrimPrefix(r.URL.Path, "/")
	if !fs.ValidPath(rel) || strings.Contains(rel, "\\") {
		http.NotFound(w, r)
		return
	}
	file := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Stat(file)
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	if immutable {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
	http.ServeFile(w, r, file)
}

func (s *Server) failure(w http.ResponseWriter, r *http.Request, status int, message string) {
	page := schema.Page{Item: "kubernetes", Version: "1.34", Title: "Documentation unavailable", Error: message, Catalog: s.catalog.Products(), Canonical: "/"}
	if query, err := parseQuery(r); err == nil {
		if _, err := s.catalog.Page(schema.Query{Item: query.Item, Version: query.Version}); err == nil {
			page.Item, page.Version = query.Item, query.Version
		}
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeJSON(w, r, status, page)
		return
	}
	s.servePage(w, r, status, page)
}

func (s *Server) servePage(w http.ResponseWriter, r *http.Request, status int, page schema.Page) {
	body := s.shell
	dynamic := true
	if status == http.StatusOK {
		if rendered, err := readRendered(filepath.Join(s.config.RenderDir, RenderFilename(page.Canonical))); err == nil {
			body = rendered
			dynamic = r.URL.RequestURI() != page.Canonical || page.Path != ""
		}
	}
	data, err := json.Marshal(page)
	if err != nil {
		http.Error(w, "Could not render documentation", http.StatusInternalServerError)
		return
	}
	body = bytes.ReplaceAll(body, []byte("<!--page-data-->"), append(append([]byte(`<script id="__PAGE_DATA__" type="application/json">`), data...), []byte("</script>")...))
	body = bytes.ReplaceAll(body, []byte("<!--app-html-->"), nil)
	if dynamic {
		body = bytes.ReplaceAll(body, []byte(`id="root"`), []byte(`id="root" data-dynamic="true"`))
	}
	canonical := s.config.SiteURL + page.Canonical
	title := html.EscapeString(page.Title + " | Manifests.io")
	description := page.Description
	if description == "" {
		description = "Browse Kubernetes and custom resource fields, types, and versions."
	}
	if len(description) > 320 {
		description = string([]rune(description)[:min(160, len([]rune(description)))])
	}
	head := `<title>` + title + `</title><meta name="description" content="` + html.EscapeString(description) + `"><link rel="canonical" href="` + html.EscapeString(canonical) + `"><meta property="og:title" content="` + title + `"><meta property="og:description" content="` + html.EscapeString(description) + `"><meta property="og:url" content="` + html.EscapeString(canonical) + `"><meta property="og:image" content="` + s.config.SiteURL + `/ogimage.png">`
	if status != http.StatusOK {
		head += `<meta name="robots" content="noindex">`
	}
	body = bytes.ReplaceAll(body, []byte("<!--page-head-->"), []byte(head))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
	digest := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(digest[:]) + `"`
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag && status == http.StatusOK {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

func readRendered(filename string) ([]byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	reader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	const maxPage = 16 << 20
	body, err := io.ReadAll(io.LimitReader(reader, maxPage+1))
	if len(body) > maxPage {
		return nil, errors.New("prerendered page exceeds size limit")
	}
	return body, err
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_ = json.NewEncoder(w).Encode(value)
	}
}
