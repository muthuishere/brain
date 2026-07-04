package engine

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
)

// FileStore persists a brain to a folder, so a git repo can BE the brain:
//
//	<dir>/episodes.ndjson    one JSON episode per line (append-only log)
//	<dir>/convictions.json   the distilled beliefs
//	<dir>/objective.json     current foreground objective + retired history
//
// Commit the folder and git history becomes the reappraisal audit trail for free.
// FileStore holds state in memory and flushes on every mutation, so each CLI
// invocation is a load → one op → flush cycle.
type FileStore struct {
	dir string
	mem *InMemoryStore
}

type objectiveFile struct {
	Current *Objective  `json:"current"`
	History []Objective `json:"history"`
	Version int         `json:"version"`
}

// NewFileStore opens (creating if needed) a brain folder and loads its state.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	fs := &FileStore{dir: dir, mem: NewInMemoryStore()}
	if err := fs.load(); err != nil {
		return nil, err
	}
	return fs, nil
}

func (fs *FileStore) path(name string) string { return filepath.Join(fs.dir, name) }

func (fs *FileStore) load() error {
	// episodes.ndjson
	if f, err := os.Open(fs.path("episodes.ndjson")); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)
		for sc.Scan() {
			line := sc.Bytes()
			if len(line) == 0 {
				continue
			}
			var ep Episode
			if err := json.Unmarshal(line, &ep); err != nil {
				return err
			}
			fs.mem.episodes = append(fs.mem.episodes, ep)
			if ep.Version > fs.mem.version {
				fs.mem.version = ep.Version
			}
		}
		if err := sc.Err(); err != nil {
			return err
		}
	}
	// convictions.json
	if data, err := os.ReadFile(fs.path("convictions.json")); err == nil {
		var cvs []Conviction
		if err := json.Unmarshal(data, &cvs); err != nil {
			return err
		}
		for _, cv := range cvs {
			fs.mem.convictions[cv.ID] = cv
		}
	}
	// objective.json
	if data, err := os.ReadFile(fs.path("objective.json")); err == nil {
		var of objectiveFile
		if err := json.Unmarshal(data, &of); err != nil {
			return err
		}
		fs.mem.objective = of.Current
		fs.mem.history = of.History
		fs.mem.objVersion = of.Version
	}
	return nil
}

func (fs *FileStore) NextVersion() int              { return fs.mem.NextVersion() }
func (fs *FileStore) Episodes() []Episode           { return fs.mem.Episodes() }
func (fs *FileStore) Convictions() []Conviction     { return fs.mem.Convictions() }
func (fs *FileStore) ObjectiveHistory() []Objective { return fs.mem.ObjectiveHistory() }
func (fs *FileStore) NextObjectiveVersion() int     { return fs.mem.NextObjectiveVersion() }

func (fs *FileStore) CurrentObjective() (Objective, bool) { return fs.mem.CurrentObjective() }

func (fs *FileStore) AddEpisode(ep Episode) {
	fs.mem.AddEpisode(ep)
	line, _ := json.Marshal(ep)
	f, err := os.OpenFile(fs.path("episodes.ndjson"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
}

func (fs *FileStore) ReplaceEpisode(ep Episode) {
	fs.mem.ReplaceEpisode(ep)
	fs.rewriteEpisodes()
}

func (fs *FileStore) rewriteEpisodes() {
	f, err := os.Create(fs.path("episodes.ndjson"))
	if err != nil {
		return
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, ep := range fs.mem.episodes {
		line, _ := json.Marshal(ep)
		_, _ = w.Write(append(line, '\n'))
	}
	_ = w.Flush()
}

func (fs *FileStore) PutConviction(cv Conviction) {
	fs.mem.PutConviction(cv)
	data, _ := json.MarshalIndent(fs.mem.Convictions(), "", "  ")
	_ = os.WriteFile(fs.path("convictions.json"), data, 0o644)
}

func (fs *FileStore) PushObjective(obj Objective) {
	fs.mem.PushObjective(obj)
	cur, _ := fs.mem.CurrentObjective()
	of := objectiveFile{Current: &cur, History: fs.mem.ObjectiveHistory(), Version: fs.mem.objVersion}
	data, _ := json.MarshalIndent(of, "", "  ")
	_ = os.WriteFile(fs.path("objective.json"), data, 0o644)
}
