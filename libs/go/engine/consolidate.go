package engine

import (
	"github.com/muthuishere/citenexus/golang/gate"

	"sort"
)

// ClusterByTokens groups episodes (with outcomes) sharing >= minShared content
// tokens, via union-find. Mirrors the Python reference deterministically.
func ClusterByTokens(episodes []Episode, minShared int) [][]Episode {
	var eps []Episode
	for _, e := range episodes {
		if e.Outcome != nil && e.Active() {
			eps = append(eps, e)
		}
	}
	n := len(eps)
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			if ra < rb {
				parent[rb] = ra
			} else {
				parent[ra] = rb
			}
		}
	}
	toks := make([]map[string]struct{}, n)
	for i, e := range eps {
		toks[i] = gate.ContentTokens(e.Cue + " " + e.Text)
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if sharedCount(toks[i], toks[j]) >= minShared {
				union(i, j)
			}
		}
	}
	groups := map[int][]Episode{}
	for idx, e := range eps {
		r := find(idx)
		groups[r] = append(groups[r], e)
	}
	out := make([][]Episode, 0, len(groups))
	for _, g := range groups {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return minVersion(out[i]) < minVersion(out[j]) })
	return out
}

func sharedCount(a, b map[string]struct{}) int {
	c := 0
	for k := range a {
		if _, ok := b[k]; ok {
			c++
		}
	}
	return c
}

func minVersion(g []Episode) int {
	m := g[0].Version
	for _, e := range g[1:] {
		if e.Version < m {
			m = e.Version
		}
	}
	return m
}

// ClusterVerdict is the deterministic ruling on whether a cluster is a conviction.
type ClusterVerdict struct {
	Valence       string
	Consistency   float64
	SupportCount  int
	SupportingIDs []string
	IsConviction  bool
	Conflicted    bool
}

// AssessCluster decides, deterministically, whether a cluster is signal or conflict.
func AssessCluster(cluster []Episode, minSupport int, minConsistency float64) ClusterVerdict {
	pos, neg := 0, 0
	for _, e := range cluster {
		if e.Outcome == nil {
			continue
		}
		switch e.Outcome.Valence() {
		case "positive":
			pos++
		case "negative":
			neg++
		}
	}
	total := pos + neg
	if total == 0 {
		return ClusterVerdict{Valence: "neutral"}
	}
	dominant := "positive"
	if neg > pos {
		dominant = "negative"
	}
	consistency := float64(max(pos, neg)) / float64(total)
	var supporting []string
	for _, e := range cluster {
		if e.Outcome != nil && e.Outcome.Valence() == dominant {
			supporting = append(supporting, e.ID)
		}
	}
	return ClusterVerdict{
		Valence:       dominant,
		Consistency:   consistency,
		SupportCount:  len(supporting),
		SupportingIDs: supporting,
		IsConviction:  len(supporting) >= minSupport && consistency >= minConsistency,
		Conflicted:    total >= minSupport && consistency < minConsistency,
	}
}
