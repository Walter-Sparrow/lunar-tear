package service

import (
	"log"
	"net/http"
	"strings"
)

const termsOfServiceHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Terms of Service</title></head>
<body>
<h1>Terms of Service</h1>
<p>Welcome to NieR Re[in]carnation. By using this application you agree to the following terms.</p>
<p>1. You agree to use this service for personal, non-commercial purposes.</p>
<p>2. All content, including characters, stories, and music, is the property of SQUARE ENIX.</p>
<p>3. You agree not to modify, reverse engineer, or redistribute any part of this application.</p>
<p>4. This service is provided as-is without warranty of any kind.</p>
<p>Last updated: 2024-04-01</p>
</body>
</html>`

const privacyPolicyHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Privacy Policy</title></head>
<body>
<h1>Privacy Policy</h1>
<p>This privacy policy describes how your personal information is collected and used.</p>
<p>1. We collect device identifiers for account management.</p>
<p>2. Game progress is stored on our servers.</p>
<p>3. We do not share your personal information with third parties.</p>
<p>Last updated: 2024-04-01</p>
</body>
</html>`

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

	// Octo asset requests
	if r.Body != nil {
		body := make([]byte, 1024)
		n, _ := r.Body.Read(body)
		if n > 0 {
			log.Printf("[HTTP]   body (%d bytes): %x", n, body[:n])
		}
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(200)
	w.Write([]byte{})
}

func (s *OctoHTTPServer) handleWebAPI(w http.ResponseWriter, r *http.Request, path string) {
	log.Printf("[WebAPI] Serving: %s", path)

	if strings.Contains(path, "database.bin") {
		// Path format: /assets/release/{version}/database.bin.e
		// Try to extract version from path and serve the matching file
		parts := strings.Split(path, "/")
		for i, p := range parts {
			if p == "release" && i+1 < len(parts) {
				version := parts[i+1]
				filePath := "assets/" + version + ".bin.e"
				log.Printf("[WebAPI] Serving master data: %s (method=%s)", filePath, r.Method)
				http.ServeFile(w, r, filePath)
				return
			}
		}
		log.Printf("[WebAPI] Serving database.bin.e fallback (method=%s)", r.Method)
		http.ServeFile(w, r, "assets/database.bin.e")
		return
	}

	if strings.Contains(path, "terms") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
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
