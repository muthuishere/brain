package engine

import (
	"crypto/sha1"
	"encoding/hex"
	"math"
	"sort"
	"strings"
)

// Objective is one foreground goal in the stream — replaced, not deleted, on change.
type Objective struct {
	Text    string  `json:"text"`
	Ts      float64 `json:"ts"`
	Version int     `json:"version"`
	AgentID string  `json:"agent_id,omitempty"`
}

// Conviction is a validated belief distilled from repeated, consistent experience.
type Conviction struct {
	ID            string   `json:"id"`
	Namespace     string   `json:"namespace"`
	Statement     string   `json:"statement"`
	Valence       string   `json:"valence"`
	SupportingIDs []string `json:"supporting_ids"`
	SupportCount  int      `json:"support_count"`
	Consistency   float64  `json:"consistency"`
	Confidence    float64  `json:"confidence"`
	GoalFrame     string   `json:"goal_frame,omitempty"`
	Ts            float64  `json:"ts"`
	Dormant       bool     `json:"dormant"`
}

// Active reports whether the conviction is in force (its goal-frame not retired).
func (c Conviction) Active() bool { return !c.Dormant }

// ConfidenceOf combines support size and consistency into [0,1). n<2 is never a
// conviction (confidence 0) — the "never act on a single unrepeated result" guard.
func ConfidenceOf(supportCount int, consistency float64) float64 {
	if supportCount < 2 {
		return 0
	}
	repetition := 1.0 - math.Pow(0.5, float64(supportCount-1))
	return consistency * repetition
}

// ConvictionID is a stable id from the belief and the episodes that support it.
func ConvictionID(namespace, statement string, supportingIDs []string) string {
	ids := append([]string(nil), supportingIDs...)
	sort.Strings(ids)
	basis := namespace + "\x00" + statement + "\x00" + strings.Join(ids, "\x00")
	sum := sha1.Sum([]byte(basis))
	return "cv_" + hex.EncodeToString(sum[:])[:16]
}
