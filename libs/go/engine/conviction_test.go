package engine

import "testing"

func TestConfidenceOf_NLessThan2IsAlwaysZero(t *testing.T) {
	for _, consistency := range []float64{0, 0.25, 0.5, 0.75, 1.0} {
		if got := ConfidenceOf(0, consistency); got != 0 {
			t.Fatalf("ConfidenceOf(0, %v) = %v, want 0", consistency, got)
		}
		if got := ConfidenceOf(1, consistency); got != 0 {
			t.Fatalf("ConfidenceOf(1, %v) = %v, want 0", consistency, got)
		}
	}
}

func TestConfidenceOf_MonotonicInSupportCount(t *testing.T) {
	const consistency = 0.8
	ns := []int{2, 3, 5, 10}
	prev := ConfidenceOf(ns[0], consistency)
	for _, n := range ns[1:] {
		cur := ConfidenceOf(n, consistency)
		if !(cur > prev) {
			t.Fatalf("expected ConfidenceOf(%d, %v)=%v > ConfidenceOf(prev, %v)=%v", n, consistency, cur, consistency, prev)
		}
		prev = cur
	}
}

func TestConfidenceOf_MonotonicInConsistency(t *testing.T) {
	const supportCount = 5
	consistencies := []float64{0.5, 0.75, 1.0}
	prev := ConfidenceOf(supportCount, consistencies[0])
	for _, c := range consistencies[1:] {
		cur := ConfidenceOf(supportCount, c)
		if !(cur > prev) {
			t.Fatalf("expected ConfidenceOf(%d, %v)=%v > ConfidenceOf(%d, prev)=%v", supportCount, c, cur, supportCount, prev)
		}
		prev = cur
	}
}

func TestConvictionID_OrderIndependentForSameInputs(t *testing.T) {
	namespace := "ns1"
	statement := "the sky is blue"
	idsA := []string{"ep_3", "ep_1", "ep_2"}
	idsB := []string{"ep_2", "ep_3", "ep_1"}

	got1 := ConvictionID(namespace, statement, idsA)
	got2 := ConvictionID(namespace, statement, idsB)

	if got1 != got2 {
		t.Fatalf("expected ConvictionID to be order-independent, got %q vs %q", got1, got2)
	}

	// Original slice must not be mutated (sort happens on a copy).
	if idsA[0] != "ep_3" || idsA[1] != "ep_1" || idsA[2] != "ep_2" {
		t.Fatalf("expected input slice to be left untouched, got %v", idsA)
	}
}

func TestConvictionID_DiffersOnDifferentNamespace(t *testing.T) {
	ids := []string{"ep_1", "ep_2"}
	id1 := ConvictionID("ns1", "same statement", ids)
	id2 := ConvictionID("ns2", "same statement", ids)
	if id1 == id2 {
		t.Fatalf("expected different namespace to produce a different ConvictionID, both = %q", id1)
	}
}

func TestConvictionID_DiffersOnDifferentStatement(t *testing.T) {
	ids := []string{"ep_1", "ep_2"}
	id1 := ConvictionID("ns1", "statement A", ids)
	id2 := ConvictionID("ns1", "statement B", ids)
	if id1 == id2 {
		t.Fatalf("expected different statement to produce a different ConvictionID, both = %q", id1)
	}
}

func TestConvictionID_DiffersOnDifferentSupportingIDs(t *testing.T) {
	id1 := ConvictionID("ns1", "same statement", []string{"ep_1", "ep_2"})
	id2 := ConvictionID("ns1", "same statement", []string{"ep_1", "ep_3"})
	if id1 == id2 {
		t.Fatalf("expected different supporting id sets to produce a different ConvictionID, both = %q", id1)
	}
}
