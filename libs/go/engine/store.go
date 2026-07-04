package engine

import "sort"

// Store is where a brain's state lives. InMemoryStore is the default; FileStore
// (store_file.go) persists to a folder so a git repo can BE the brain. The Store
// owns the monotonic version counter — every episode gets a deterministic
// freshness stamp ("don't ask the LLM which fact is current").
type Store interface {
	NextVersion() int
	AddEpisode(ep Episode)
	ReplaceEpisode(ep Episode)
	Episodes() []Episode

	PutConviction(cv Conviction)
	Convictions() []Conviction

	CurrentObjective() (Objective, bool)
	ObjectiveHistory() []Objective
	PushObjective(obj Objective) // sets current, retiring the previous to history
	NextObjectiveVersion() int
}

// InMemoryStore is the reference store — everything in RAM, nothing persisted.
type InMemoryStore struct {
	episodes    []Episode
	convictions map[string]Conviction
	objective   *Objective
	history     []Objective
	version     int
	objVersion  int
}

// NewInMemoryStore builds an empty in-memory store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{convictions: map[string]Conviction{}}
}

func (s *InMemoryStore) NextVersion() int { s.version++; return s.version }

func (s *InMemoryStore) AddEpisode(ep Episode) { s.episodes = append(s.episodes, ep) }

func (s *InMemoryStore) ReplaceEpisode(ep Episode) {
	for i := range s.episodes {
		if s.episodes[i].ID == ep.ID {
			s.episodes[i] = ep
			return
		}
	}
}

func (s *InMemoryStore) Episodes() []Episode { return s.episodes }

func (s *InMemoryStore) PutConviction(cv Conviction) { s.convictions[cv.ID] = cv }

func (s *InMemoryStore) Convictions() []Conviction {
	out := make([]Conviction, 0, len(s.convictions))
	for _, cv := range s.convictions {
		out = append(out, cv)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Confidence > out[j].Confidence })
	return out
}

func (s *InMemoryStore) CurrentObjective() (Objective, bool) {
	if s.objective == nil {
		return Objective{}, false
	}
	return *s.objective, true
}

func (s *InMemoryStore) ObjectiveHistory() []Objective { return s.history }

func (s *InMemoryStore) PushObjective(obj Objective) {
	if s.objective != nil {
		s.history = append(s.history, *s.objective)
	}
	s.objective = &obj
}

func (s *InMemoryStore) NextObjectiveVersion() int { s.objVersion++; return s.objVersion }
