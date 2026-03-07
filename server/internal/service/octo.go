package service

import (
	"log"
	"net/http"
	"os"
	"strings"
)

const termsOfServiceHTML = `<html><head><title>Terms of Service</title></head><body>###1###</body></html>`
const privacyPolicyHTML = `<html><head><title>Privacy Policy</title></head><body>###2###</body></html>`

type OctoHTTPServer struct {
	mux *http.ServeMux
}

func NewOctoHTTPServer() *OctoHTTPServer {
	s := &OctoHTTPServer{mux: http.NewServeMux()}
	s.mux.HandleFunc("/", s.handleAll)
	return s
}

func (s *OctoHTTPServer) Handler() http.Handler {
	return s.mux
}

func (s *OctoHTTPServer) handleAll(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HTTP] %s %s (Host: %s)", r.Method, r.URL.String(), r.Host)
	for k, v := range r.Header {
		log.Printf("[HTTP]   %s: %s", k, v)
	}

	path := r.URL.Path

	// Octo v2 API — asset bundle management
	if strings.HasPrefix(path, "/v2/") {
		s.handleOctoV2(w, r, path)
		return
	}

	// Octo v1 list: /v1/list/{version}/{revision} — same list.bin as v2, keyed by revision
	if strings.HasPrefix(path, "/v1/list/") {
		s.serveOctoV1List(w, r, path)
		return
	}

	// Game web API requests
	if strings.Contains(path, "/web/") || strings.Contains(r.Host, "web.app.nierreincarnation") {
		s.handleWebAPI(w, r, path)
		return
	}

	// Master data download (should not be reached if version matches)
	if strings.HasPrefix(path, "/master-data/") {
		log.Printf("[HTTP] Master data request for path: %s — returning empty", path)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(200)
		return
	}

	// /assets/release/{version}/database.bin.e — master data (HEAD/GET), same as MariesWonderland
	if strings.Contains(path, "/assets/release/") && strings.Contains(path, "database.bin") {
		s.serveDatabaseBinE(w, r, path)
		return
	}

	// Log request body for debugging Octo protocol
	if r.Body != nil {
		body := make([]byte, 4096)
		n, _ := r.Body.Read(body)
		if n > 0 {
			log.Printf("[HTTP]   body (%d bytes): %x", n, body[:n])
			if n < 256 {
				log.Printf("[HTTP]   body (ascii): %s", string(body[:n]))
			}
		}
	}

	log.Printf("[HTTP] >>> UNHANDLED REQUEST: %s %s — returning empty 200", r.Method, path)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(200)
	w.Write([]byte{})
}

func (s *OctoHTTPServer) handleOctoV2(w http.ResponseWriter, r *http.Request, path string) {
	log.Printf("[OctoV2] %s %s", r.Method, path)

	// /v2/pub/a/{appId}/v/{version}/list/{offset} — resource listing
	if strings.Contains(path, "/list/") {
		parts := strings.Split(path, "/")
		if len(parts) > 0 {
			revision := parts[len(parts)-1]
			if revision != "" {
				filePath := "assets/revisions/" + revision + "/list.bin"
				log.Printf("[OctoV2] Resource list request — serving %s (revision=%s)", filePath, revision)
				w.Header().Set("Content-Type", "application/x-protobuf")
				http.ServeFile(w, r, filePath)
				return
			}
		}

		log.Printf("[OctoV2] Resource list request without revision segment — returning empty protobuf")
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
		return
	}

	// /v2/pub/a/{appId}/v/{version}/info — DB info
	if strings.Contains(path, "/info") {
		log.Printf("[OctoV2] Info request — returning empty protobuf")
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(200)
		return
	}

	log.Printf("[OctoV2] Unknown endpoint: %s — returning empty protobuf", path)
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(200)
}

// serveOctoV1List handles GET /v1/list/{version}/{revision} — serves assets/revisions/{revision}/list.bin.
func (s *OctoHTTPServer) serveOctoV1List(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// ["v1", "list", "300116832", "0"] -> revision = last segment
	revision := "0"
	if len(parts) >= 4 {
		revision = parts[len(parts)-1]
	}
	filePath := "assets/revisions/" + revision + "/list.bin"
	if _, err := os.Stat(filePath); err != nil {
		log.Printf("[OctoV1] list not found: %s, falling back to revision 0", filePath)
		filePath = "assets/revisions/0/list.bin"
	}
	log.Printf("[OctoV1] %s %s — serving %s", r.Method, path, filePath)
	w.Header().Set("Content-Type", "application/x-protobuf")
	http.ServeFile(w, r, filePath)
}

// serveDatabaseBinE serves MasterMemory database: /assets/release/{version}/database.bin.e
// -> assets/release/{version}.bin.e (or assets/release/database.bin.e fallback).
func (s *OctoHTTPServer) serveDatabaseBinE(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.Split(path, "/")
	var version string
	for i, p := range parts {
		if p == "release" && i+1 < len(parts) {
			version = parts[i+1]
			break
		}
	}
	filePath := "assets/release/database.bin.e"
	if version != "" {
		vPath := "assets/release/" + version + ".bin.e"
		if _, err := os.Stat(vPath); err == nil {
			filePath = vPath
		}
	}
	log.Printf("[WebAPI] Serving master data: %s (method=%s)", filePath, r.Method)
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, filePath)
}

func (s *OctoHTTPServer) handleWebAPI(w http.ResponseWriter, r *http.Request, path string) {
	log.Printf("[WebAPI] Serving: %s", path)

	if strings.Contains(path, "database.bin") {
		s.serveDatabaseBinE(w, r, path)
		return
	}

	if strings.Contains(path, "termsofuse") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(200)
		w.Write([]byte(termsOfServiceHTML))
		return
	}

	if strings.Contains(path, "privacy") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte(privacyPolicyHTML))
		return
	}

	if strings.Contains(path, "maintenance") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte(`<!DOCTYPE html><html><body></body></html>`))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte(`<!DOCTYPE html><html><body></body></html>`))
}
