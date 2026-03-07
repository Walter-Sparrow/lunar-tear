package service

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const termsOfServiceHTML = `<html><head><title>Terms of Service</title></head><body>###1###</body></html>`
const privacyPolicyHTML = `<html><head><title>Privacy Policy</title></head><body>###2###</body></html>`

// resourcesURLOriginal is the base URL embedded in list.bin; must be replaced with same-length (43 bytes) when rewriting.
const resourcesURLOriginal = "https://resources.app.nierreincarnation.com"

type OctoHTTPServer struct {
	mux              *http.ServeMux
	ResourcesBaseURL string // if non-empty and exactly 43 chars, list.bin is rewritten to use this base for asset URLs
	revisions        *revisionTracker
	resolver         *assetResolver
}

// countResponseWriter wraps http.ResponseWriter and counts bytes written.
type countResponseWriter struct {
	http.ResponseWriter
	n int64
}

type fileMD5Entry struct {
	size    int64
	modTime int64
	md5     string
}

var (
	fileMD5Cache   = make(map[string]fileMD5Entry)
	fileMD5CacheMu sync.RWMutex
)

func (c *countResponseWriter) Write(p []byte) (int, error) {
	n, err := c.ResponseWriter.Write(p)
	c.n += int64(n)
	return n, err
}

func fileMD5Hex(path string, info os.FileInfo) (string, error) {
	modTime := info.ModTime().UnixNano()

	fileMD5CacheMu.RLock()
	cached, ok := fileMD5Cache[path]
	fileMD5CacheMu.RUnlock()
	if ok && cached.size == info.Size() && cached.modTime == modTime {
		return cached.md5, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	sum := hex.EncodeToString(h.Sum(nil))

	fileMD5CacheMu.Lock()
	fileMD5Cache[path] = fileMD5Entry{
		size:    info.Size(),
		modTime: modTime,
		md5:     sum,
	}
	fileMD5CacheMu.Unlock()
	return sum, nil
}

func NewOctoHTTPServer(resourcesBaseURL string) *OctoHTTPServer {
	s := &OctoHTTPServer{
		mux:              http.NewServeMux(),
		ResourcesBaseURL: resourcesBaseURL,
		revisions:        newRevisionTracker(),
		resolver:         newAssetResolver(),
	}
	s.mux.HandleFunc("/", s.handleAll)
	return s
}

func (s *OctoHTTPServer) Handler() http.Handler {
	return s.mux
}

func (s *OctoHTTPServer) handleAll(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	isAssetRequest := strings.HasPrefix(path, "/resource-bundle-server/") || strings.Contains(path, "/unso-")
	isMasterDataRequest := strings.Contains(path, "/assets/release/") && strings.Contains(path, "database.bin")
	if !isAssetRequest && !isMasterDataRequest {
		log.Printf("[HTTP] %s %s (Host: %s)", r.Method, r.URL.String(), r.Host)
		for k, v := range r.Header {
			log.Printf("[HTTP]   %s: %s", k, v)
		}
	}

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

	// Asset bundle requests (from list.bin URLs: .../unso-{v}-{type}/{o}?generation=...&alt=media)
	if strings.Contains(path, "/unso-") {
		s.serveUnsoAsset(w, r, path)
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
			requestedRevision := parts[len(parts)-1]
			if requestedRevision != "" {
				revision := "0"
				filePath := "assets/revisions/0/list.bin"
				if requestedRevision != revision {
					log.Printf("[OctoV2] Resource list request revision=%s canonicalized to revision=%s", requestedRevision, revision)
				}
				log.Printf("[OctoV2] Resource list request — serving %s (requested_revision=%s canonical_revision=%s)", filePath, requestedRevision, revision)
				s.revisions.Remember(r.RemoteAddr, revision)
				go s.resolver.Prewarm(revision)
				s.serveListBin(w, filePath)
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
	requestedRevision := "0"
	if len(parts) >= 4 {
		requestedRevision = parts[len(parts)-1]
	}
	revision := "0"
	filePath := "assets/revisions/0/list.bin"
	if requestedRevision != revision {
		log.Printf("[OctoV1] list request revision=%s canonicalized to revision=%s", requestedRevision, revision)
	}
	log.Printf("[OctoV1] %s %s — serving %s (requested_revision=%s canonical_revision=%s)", r.Method, path, filePath, requestedRevision, revision)
	s.revisions.Remember(r.RemoteAddr, revision)
	go s.resolver.Prewarm(revision)
	s.serveListBin(w, filePath)
}

// serveUnsoAsset serves asset bundle or resource for URLs like /resource-bundle-server/unso-{version}-{type}/{object_id}.
func (s *OctoHTTPServer) serveUnsoAsset(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	var segment, objectID string
	for i, p := range parts {
		if strings.HasPrefix(p, "unso-") && i+1 < len(parts) {
			segment = p
			objectID = parts[i+1]
			break
		}
	}
	if segment == "" || objectID == "" {
		log.Printf("[HTTP] Asset request malformed: %s", path)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	// segment = "unso-200116832-assetbundle" -> type = last part after "-"
	segParts := strings.Split(segment, "-")
	if len(segParts) < 2 {
		log.Printf("[HTTP] Asset request segment malformed: %s", segment)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	assetType := segParts[len(segParts)-1] // "assetbundle" or "resources"
	if assetType != "assetbundle" && assetType != "resources" {
		log.Printf("[HTTP] Asset request unknown type: %s", assetType)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	activeRevision := s.revisions.Active(r.RemoteAddr)
	resolution, ok := s.resolver.Resolve(objectID, assetType, activeRevision)
	if !ok {
		log.Printf("[HTTP] Asset not found: %s (object_id=%s type=%s active_revision=%s) no candidates", path, objectID, assetType, activeRevision)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	baseDir := filepath.Join("assets", "revisions")
	var triedPaths []string
	var md5Mismatches []string
	for _, candidate := range resolution.Candidates {
		rel, err := filepath.Rel(baseDir, candidate.Path)
		if err != nil || strings.Contains(rel, "..") || filepath.IsAbs(rel) {
			continue
		}
		triedPaths = append(triedPaths, candidate.Revision+":"+candidate.Path+" ["+candidate.Source+"]")
		f, err := os.Open(candidate.Path)
		if err != nil {
			continue
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			continue
		}
		if info.IsDir() {
			f.Close()
			continue
		}
		// Only validate size when list.bin gave a plausible file size (>= 256); small values are often wrong (e.g. different proto field).
		if resolution.ListSize >= 256 && info.Size() != resolution.ListSize {
			f.Close()
			continue
		}
		if candidate.ExpectedMD5 != "" {
			actualMD5, err := fileMD5Hex(candidate.Path, info)
			if err != nil {
				log.Printf("[HTTP] Asset md5 read failed: %s err=%v", candidate.Path, err)
				f.Close()
				continue
			}
			if !strings.EqualFold(actualMD5, candidate.ExpectedMD5) {
				md5Mismatches = append(md5Mismatches, candidate.Revision+":"+candidate.Path+" ["+candidate.Source+"] expected="+candidate.ExpectedMD5+" actual="+actualMD5)
				log.Printf("[HTTP] Asset md5 mismatch: object_id=%s type=%s path=%s expected=%s actual=%s active_revision=%s list_revision=%s resolved_revision=%s source=%s", objectID, assetType, candidate.Path, candidate.ExpectedMD5, actualMD5, resolution.ActiveRevision, resolution.ListRevision, candidate.Revision, candidate.Source)
				f.Close()
				continue
			}
		}
		defer f.Close()
		w.Header().Set("Content-Type", "application/octet-stream")
		cw := &countResponseWriter{ResponseWriter: w}
		http.ServeContent(cw, r, filepath.Base(candidate.Path), info.ModTime(), f)
		return
	}
	if len(md5Mismatches) > 0 {
		log.Printf("[HTTP] Asset md5 mismatches: object_id=%s type=%s active_revision=%s list_revision=%s mismatches=%v", objectID, assetType, resolution.ActiveRevision, resolution.ListRevision, md5Mismatches)
	}
	log.Printf("[HTTP] Asset not found: %s (object_id=%s type=%s active_revision=%s list_revision=%s) tried paths: %v", path, objectID, assetType, resolution.ActiveRevision, resolution.ListRevision, triedPaths)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusNotFound)
}

// serveListBin reads list.bin from filePath, optionally rewrites the resource base URL to s.ResourcesBaseURL
// (must be exactly 43 bytes to preserve protobuf layout), and writes the result to w.
func (s *OctoHTTPServer) serveListBin(w http.ResponseWriter, filePath string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("[Octo] list.bin read error: %v", err)
		http.Error(w, "list not found", http.StatusNotFound)
		return
	}
	orig := []byte(resourcesURLOriginal)
	if s.ResourcesBaseURL != "" {
		if len(s.ResourcesBaseURL) != len(orig) {
			log.Printf("[Octo] resources-base-url length is %d, need %d — serving list.bin unchanged", len(s.ResourcesBaseURL), len(orig))
		} else {
			repl := []byte(s.ResourcesBaseURL)
			if idx := bytes.Index(data, orig); idx >= 0 {
				copy(data[idx:], repl)
				log.Printf("[Octo] list.bin: rewrote resource base URL to %s", s.ResourcesBaseURL)
			}
		}
	}
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
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
