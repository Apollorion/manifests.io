package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/TheOutdoorProgrammer/manifests.io/internal/schema"
)

const (
	sitemapNamespace  = "http://www.sitemaps.org/schemas/sitemap/0.9"
	sitemapMaxEntries = 50_000
	sitemapMaxBytes   = 50 * 1024 * 1024
)

type crawlerFile struct {
	body        []byte
	contentType string
	etag        string
}

func buildCrawlerFiles(site string, routes []schema.Query) (map[string]crawlerFile, error) {
	locations := make([]string, 0, len(routes))
	for _, route := range routes {
		canonical := schema.Query{Item: route.Item, Version: route.Version, Resource: route.Resource, Pointer: route.Pointer}
		location := site + schema.Href(canonical)
		// The sitemap protocol cannot represent locations of 2,048 characters or more.
		if len(location) < 2048 {
			locations = append(locations, location)
		}
	}
	slices.Sort(locations)
	locations = slices.Compact(locations)
	chunks, err := sitemapDocuments("urlset", "url", locations, sitemapMaxEntries, sitemapMaxBytes)
	if err != nil {
		return nil, err
	}
	files := make(map[string]crawlerFile, len(chunks)+2)
	add := func(path, contentType string, body []byte) {
		digest := sha256.Sum256(body)
		files[path] = crawlerFile{body: body, contentType: contentType, etag: `"` + hex.EncodeToString(digest[:]) + `"`}
	}
	indexes := make([]string, len(chunks))
	for i, chunk := range chunks {
		path := "/sitemap-" + strconv.Itoa(i+1) + ".xml"
		indexes[i] = site + path
		add(path, "application/xml; charset=utf-8", chunk)
	}
	index, err := sitemapDocuments("sitemapindex", "sitemap", indexes, sitemapMaxEntries, sitemapMaxBytes)
	if err != nil {
		return nil, err
	}
	if len(index) != 1 {
		return nil, fmt.Errorf("sitemap index exceeds protocol limits")
	}
	add("/sitemap.xml", "application/xml; charset=utf-8", index[0])
	add("/robots.txt", "text/plain; charset=utf-8", []byte("User-agent: *\nDisallow:\n\nSitemap: "+site+"/sitemap.xml\n"))
	return files, nil
}

func sitemapDocuments(root, entry string, locations []string, maxEntries, maxBytes int) ([][]byte, error) {
	header := []byte(xml.Header + `<` + root + ` xmlns="` + sitemapNamespace + `">`)
	footer := []byte(`</` + root + `>`)
	var documents [][]byte
	var body bytes.Buffer
	body.Write(header)
	count := 0
	finish := func() {
		body.Write(footer)
		documents = append(documents, bytes.Clone(body.Bytes()))
		body.Reset()
		body.Write(header)
		count = 0
	}
	for _, location := range locations {
		item, err := xml.Marshal(struct {
			XMLName  xml.Name
			Location string `xml:"loc"`
		}{XMLName: xml.Name{Local: entry}, Location: location})
		if err != nil {
			return nil, err
		}
		if len(header)+len(item)+len(footer) > maxBytes {
			return nil, fmt.Errorf("sitemap entry exceeds document size limit")
		}
		if count == maxEntries || body.Len()+len(item)+len(footer) > maxBytes {
			finish()
		}
		body.Write(item)
		count++
	}
	if count > 0 || len(documents) == 0 {
		finish()
	}
	return documents, nil
}

func (s *Server) serveCrawler(w http.ResponseWriter, r *http.Request) {
	file, ok := s.crawlers[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", file.contentType)
	w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
	w.Header().Set("ETag", file.etag)
	if r.Header.Get("If-None-Match") == file.etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(file.body)))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(file.body)
	}
}
