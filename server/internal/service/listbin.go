package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// listBinEntry holds path (')' as segment separator) and size from list.bin; Size is 0 when not present.
type listBinEntry struct {
	Path string
	Size int64
	MD5  string
}

// listBinIndex caches object_id → entry for a revision.
type listBinIndex map[string]listBinEntry

type infoAlias struct {
	ToName     string
	ToRevision string
	MD5        string
}

type assetCandidate struct {
	Path        string
	Revision    string
	Source      string
	ExpectedMD5 string
}

type listBinLoad struct {
	done chan struct{}
	idx  listBinIndex
	ok   bool
}

type infoLoad struct {
	done chan struct{}
	m    map[string]infoAlias
}

var (
	listBinCache    = make(map[string]listBinIndex) // revision → index
	listBinInflight = make(map[string]*listBinLoad)
	listBinCacheMu  sync.RWMutex
	infoCache       = make(map[string]map[string]infoAlias) // revision → from-name → duplicate target
	infoInflight    = make(map[string]*infoLoad)
	infoCacheMu     sync.RWMutex
)

// infoJSONEntry is one entry from assets/revisions/{rev}/info.json (duplicate files: serve to-name when asked for from-name).
type infoJSONEntry struct {
	FromName   string `json:"from-name"`
	ToName     string `json:"to-name"`
	ToRevision *int   `json:"to-revision"`
	MD5        string `json:"md5"`
}

// readVarint reads a protobuf varint from b, returns value and number of bytes consumed.
func readVarint(b []byte) (value int, n int) {
	for i := 0; i < len(b) && i < 10; i++ {
		value |= int(b[i]&0x7f) << (7 * i)
		n = i + 1
		if b[i]&0x80 == 0 {
			return value, n
		}
	}
	return 0, 0
}

func skipProtoField(wireType int, data []byte, offset int) (int, bool) {
	switch wireType {
	case 0:
		_, n := readVarint(data[offset:])
		if n == 0 {
			return 0, false
		}
		return offset + n, true
	case 1:
		if offset+8 > len(data) {
			return 0, false
		}
		return offset + 8, true
	case 2:
		length, n := readVarint(data[offset:])
		if n == 0 || length < 0 || offset+n+length > len(data) {
			return 0, false
		}
		return offset + n + length, true
	case 5:
		if offset+4 > len(data) {
			return 0, false
		}
		return offset + 4, true
	default:
		return 0, false
	}
}

func parseListBinEntry(data []byte) (objectID string, entry listBinEntry, ok bool) {
	i := 0
	for i < len(data) {
		tag, n := readVarint(data[i:])
		if n == 0 {
			return "", listBinEntry{}, false
		}
		i += n
		fieldNum := tag >> 3
		wireType := tag & 0x7

		switch fieldNum {
		case 3: // path
			if wireType != 2 {
				return "", listBinEntry{}, false
			}
			length, vn := readVarint(data[i:])
			if vn == 0 || length < 0 || i+vn+length > len(data) {
				return "", listBinEntry{}, false
			}
			entry.Path = string(data[i+vn : i+vn+length])
			i += vn + length
		case 4: // size
			if wireType != 0 {
				return "", listBinEntry{}, false
			}
			size, vn := readVarint(data[i:])
			if vn == 0 {
				return "", listBinEntry{}, false
			}
			if size >= 256 {
				entry.Size = int64(size)
			}
			i += vn
		case 10: // md5
			if wireType != 2 {
				return "", listBinEntry{}, false
			}
			length, vn := readVarint(data[i:])
			if vn == 0 || length < 0 || i+vn+length > len(data) {
				return "", listBinEntry{}, false
			}
			entry.MD5 = string(data[i+vn : i+vn+length])
			i += vn + length
		case 11: // object_id
			if wireType != 2 {
				return "", listBinEntry{}, false
			}
			length, vn := readVarint(data[i:])
			if vn == 0 || length <= 0 || i+vn+length > len(data) {
				return "", listBinEntry{}, false
			}
			objectID = string(data[i+vn : i+vn+length])
			i += vn + length
		default:
			next, ok := skipProtoField(wireType, data, i)
			if !ok {
				return "", listBinEntry{}, false
			}
			i = next
		}
	}

	if objectID == "" || entry.Path == "" {
		return "", listBinEntry{}, false
	}
	return objectID, entry, true
}

// parseListBin reads list.bin and builds object_id (6-byte string) → entry (path, size, md5).
// The file is a protobuf message with repeated nested entry messages, so we parse each entry
// boundary first instead of doing a flat scan across the whole file.
func parseListBin(data []byte) listBinIndex {
	idx := make(listBinIndex)
	i := 0
	for i < len(data) {
		tag, n := readVarint(data[i:])
		if n == 0 {
			break
		}
		i += n
		fieldNum := tag >> 3
		wireType := tag & 0x7

		if fieldNum == 2 && wireType == 2 {
			length, vn := readVarint(data[i:])
			if vn == 0 || length < 0 || i+vn+length > len(data) {
				break
			}
			entryBytes := data[i+vn : i+vn+length]
			objectID, entry, ok := parseListBinEntry(entryBytes)
			if ok {
				idx[objectID] = entry
			}
			i += vn + length
			continue
		}

		next, ok := skipProtoField(wireType, data, i)
		if !ok {
			break
		}
		i = next
	}
	return idx
}

// listRevisions returns revision dir names that have list.bin, sorted descending (newer first).
func listRevisions() []string {
	dir := filepath.Join("assets", "revisions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{"0"}
	}
	var revs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "" || name == "." || name == ".." {
			continue
		}
		listPath := filepath.Join(dir, name, "list.bin")
		if _, err := os.Stat(listPath); err != nil {
			continue
		}
		revs = append(revs, name)
	}
	if len(revs) == 0 {
		return []string{"0"}
	}
	sort.Slice(revs, func(i, j int) bool {
		// Prefer numeric sort descending so 817 is tried before 0.
		ri, errI := strconv.Atoi(revs[i])
		rj, errJ := strconv.Atoi(revs[j])
		if errI == nil && errJ == nil {
			return ri > rj
		}
		return revs[i] > revs[j]
	})
	return revs
}

// loadListBinIndex loads or returns cached index for the given revision.
func loadListBinIndex(revision string) (listBinIndex, bool) {
	listBinCacheMu.RLock()
	idx, ok := listBinCache[revision]
	listBinCacheMu.RUnlock()
	if ok {
		return idx, true
	}

	listBinCacheMu.Lock()
	if idx, ok := listBinCache[revision]; ok {
		listBinCacheMu.Unlock()
		return idx, true
	}
	if load := listBinInflight[revision]; load != nil {
		listBinCacheMu.Unlock()
		<-load.done
		return load.idx, load.ok
	}
	load := &listBinLoad{done: make(chan struct{})}
	listBinInflight[revision] = load
	listBinCacheMu.Unlock()

	filePath := filepath.Join("assets", "revisions", revision, "list.bin")
	data, err := os.ReadFile(filePath)
	if err != nil {
		listBinCacheMu.Lock()
		delete(listBinInflight, revision)
		close(load.done)
		listBinCacheMu.Unlock()
		return nil, false
	}
	idx = parseListBin(data)
	load.idx = idx
	load.ok = true
	listBinCacheMu.Lock()
	listBinCache[revision] = idx
	delete(listBinInflight, revision)
	close(load.done)
	listBinCacheMu.Unlock()
	return idx, true
}

// loadInfoIndex loads or returns cached from-name → duplicate target map from info.json.
// Returns nil if file missing or invalid; caller should treat as no mappings.
func loadInfoIndex(revision string) map[string]infoAlias {
	infoCacheMu.RLock()
	m, ok := infoCache[revision]
	infoCacheMu.RUnlock()
	if ok {
		return m
	}

	infoCacheMu.Lock()
	if m, ok := infoCache[revision]; ok {
		infoCacheMu.Unlock()
		return m
	}
	if load := infoInflight[revision]; load != nil {
		infoCacheMu.Unlock()
		<-load.done
		return load.m
	}
	load := &infoLoad{done: make(chan struct{})}
	infoInflight[revision] = load
	infoCacheMu.Unlock()

	filePath := filepath.Join("assets", "revisions", revision, "info.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		infoCacheMu.Lock()
		infoCache[revision] = nil
		delete(infoInflight, revision)
		close(load.done)
		infoCacheMu.Unlock()
		return nil
	}
	var entries []infoJSONEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		infoCacheMu.Lock()
		infoCache[revision] = nil
		delete(infoInflight, revision)
		close(load.done)
		infoCacheMu.Unlock()
		return nil
	}
	m = make(map[string]infoAlias)
	for _, e := range entries {
		if e.FromName != "" && e.ToName != "" {
			aliasRevision := revision
			if e.ToRevision != nil {
				aliasRevision = strconv.Itoa(*e.ToRevision)
			}
			m[e.FromName] = infoAlias{
				ToName:     e.ToName,
				ToRevision: aliasRevision,
				MD5:        e.MD5,
			}
		}
	}
	load.m = m
	infoCacheMu.Lock()
	infoCache[revision] = m
	delete(infoInflight, revision)
	close(load.done)
	infoCacheMu.Unlock()
	return m
}

// pathStrToFullPaths builds full file paths for a list.bin path string (')' as segment separator).
// Returns the primary path and fallbacks with locale segments ja/ko replaced by en for "global" variant.
func pathStrToFullPaths(revision, assetType, pathStr string) []string {
	fsPath := strings.ReplaceAll(pathStr, ")", "/")
	if strings.Contains(fsPath, "..") || filepath.IsAbs(fsPath) || strings.HasPrefix(fsPath, "/") {
		return nil
	}
	fsPath = filepath.Clean(fsPath)
	if strings.Contains(fsPath, "..") {
		return nil
	}
	// Prefer "global" (en) when list.bin points to ja/ko: try en first, then original.
	var pathStrs []string
	if strings.Contains(pathStr, ")ja)") {
		pathStrs = append(pathStrs, strings.ReplaceAll(pathStr, ")ja)", ")en)"))
	}
	if strings.Contains(pathStr, ")ko)") {
		pathStrs = append(pathStrs, strings.ReplaceAll(pathStr, ")ko)", ")en)"))
	}
	pathStrs = append(pathStrs, pathStr)
	base := filepath.Join("assets", "revisions", revision)
	var out []string
	seen := make(map[string]bool)
	for _, p := range pathStrs {
		cleaned := filepath.Clean(strings.ReplaceAll(p, ")", "/"))
		if seen[cleaned] {
			continue
		}
		seen[cleaned] = true
		switch assetType {
		case "assetbundle":
			out = append(out, filepath.Join(base, "assetbundle", cleaned+".assetbundle"))
		case "resources":
			out = append(out, filepath.Join(base, "resources", cleaned))
		default:
			return nil
		}
	}
	return out
}

func appendUniqueCandidate(candidates []assetCandidate, seen map[string]bool, candidate assetCandidate) []assetCandidate {
	key := candidate.Revision + ":" + candidate.Path
	if seen[key] {
		return candidates
	}
	seen[key] = true
	return append(candidates, candidate)
}

func duplicateCandidatePath(candidate assetCandidate, assetType, targetRevision, targetBaseName string) string {
	root := filepath.Join("assets", "revisions", candidate.Revision, assetType)
	rel, err := filepath.Rel(root, candidate.Path)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return ""
	}
	return filepath.Join("assets", "revisions", targetRevision, assetType, filepath.Dir(rel), targetBaseName)
}

// objectIDToFilePath resolves object_id and type (assetbundle|resources) to a file path under assets/revisions/{revision}/
// and the size from list.bin (0 if not present). pathFromList uses ')' as segment separator; we convert to '/' and add extension for assetbundle.
// Returns empty path and false if not found or path would escape (traversal).
func objectIDToFilePath(revision, assetType, objectID string) (filePath string, size int64, ok bool) {
	candidates, size, ok := objectIDToFilePathCandidates(revision, assetType, objectID)
	if !ok || len(candidates) == 0 {
		return "", 0, false
	}
	return candidates[0].Path, size, true
}

// objectIDToFilePathCandidates returns candidate file paths for the object: list.bin path, locale fallbacks
// (ja/ko -> en), then info.json duplicate mappings (from-name -> to-name, possibly in a different revision).
// Callers should try each path until one exists on disk.
func objectIDToFilePathCandidates(revision, assetType, objectID string) (candidates []assetCandidate, size int64, ok bool) {
	idx, ok := loadListBinIndex(revision)
	if !ok || idx == nil {
		return nil, 0, false
	}
	entry, ok := idx[objectID]
	if !ok || entry.Path == "" {
		return nil, 0, false
	}
	paths := pathStrToFullPaths(revision, assetType, entry.Path)
	if len(paths) == 0 {
		return nil, 0, false
	}
	seen := make(map[string]bool)
	for _, path := range paths {
		candidates = appendUniqueCandidate(candidates, seen, assetCandidate{
			Path:        path,
			Revision:    revision,
			Source:      "list.bin",
			ExpectedMD5: entry.MD5,
		})
	}
	// Add paths from info.json: when requested file is a "from-name" (duplicate not included), serve "to-name" instead.
	infoIndex := loadInfoIndex(revision)
	if len(infoIndex) > 0 {
		for _, c := range candidates {
			alias, ok := infoIndex[filepath.Base(c.Path)]
			if !ok || alias.ToName == "" {
				continue
			}
			alt := duplicateCandidatePath(c, assetType, alias.ToRevision, alias.ToName)
			if alt == "" {
				continue
			}
			candidates = appendUniqueCandidate(candidates, seen, assetCandidate{
				Path:        alt,
				Revision:    alias.ToRevision,
				Source:      "info.json redirect",
				ExpectedMD5: alias.MD5,
			})
		}
	}
	return candidates, entry.Size, true
}
