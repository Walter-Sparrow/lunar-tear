package service

import (
	"log"
	"net"
	"sync"
	"time"
)

type revisionTracker struct {
	mu             sync.RWMutex
	activeByClient map[string]string
	lastRevision   string
}

type assetResolution struct {
	ActiveRevision string
	ListRevision   string
	ListSize       int64
	Candidates     []assetCandidate
}

type assetResolver struct{}

func newRevisionTracker() *revisionTracker {
	return &revisionTracker{
		activeByClient: make(map[string]string),
	}
}

func newAssetResolver() *assetResolver {
	return &assetResolver{}
}

func normalizeClientAddr(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil && host != "" {
		return host
	}
	return remoteAddr
}

func (t *revisionTracker) Remember(clientAddr, revision string) {
	if revision == "" {
		return
	}
	client := normalizeClientAddr(clientAddr)
	t.mu.Lock()
	if client != "" {
		t.activeByClient[client] = revision
	}
	t.lastRevision = revision
	t.mu.Unlock()
	log.Printf("[Octo] Active list revision for client=%s set to %s", client, revision)
}

func (t *revisionTracker) Active(clientAddr string) string {
	client := normalizeClientAddr(clientAddr)
	t.mu.RLock()
	revision := t.activeByClient[client]
	if revision == "" {
		revision = t.lastRevision
	}
	t.mu.RUnlock()
	if revision == "" {
		return "0"
	}
	return revision
}

func (r *assetResolver) Resolve(objectID, assetType, activeRevision string) (assetResolution, bool) {
	start := time.Now()
	resolution := assetResolution{ActiveRevision: activeRevision}
	revision := activeRevision

	candidates, listSize, ok := objectIDToFilePathCandidates(revision, assetType, objectID)
	if ok && len(candidates) > 0 {
		resolution.ListRevision = revision
		resolution.ListSize = listSize
		resolution.Candidates = candidates
		if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
			log.Printf("[HTTP] Asset resolve slow: object_id=%s type=%s active_revision=%s list_revision=%s elapsed=%s", objectID, assetType, activeRevision, revision, elapsed)
		}
		return resolution, true
	}

	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		log.Printf("[HTTP] Asset resolve miss: object_id=%s type=%s active_revision=%s elapsed=%s", objectID, assetType, activeRevision, elapsed)
	}
	return resolution, false
}

func (r *assetResolver) Prewarm(activeRevision string) {
	if activeRevision == "" {
		return
	}
	_, _ = loadListBinIndex(activeRevision)
	_ = loadInfoIndex(activeRevision)
}
