package interner

import "testing"

func TestInternStatsCountDuplicateSavings(t *testing.T) {
	Reset()
	const value = "duplicate-value"
	if got := Intern(value); got != value {
		t.Fatalf("Intern() = %q, want %q", got, value)
	}
	if got := Intern(value); got != value {
		t.Fatalf("second Intern() = %q, want %q", got, value)
	}
	if hits := global.hits.Load(); hits != 1 {
		t.Fatalf("hits = %d, want 1", hits)
	}
	if misses := global.misses.Load(); misses != 1 {
		t.Fatalf("misses = %d, want 1", misses)
	}
	if savings := global.savings.Load(); savings != int64(len(value)) {
		t.Fatalf("savings = %d, want %d", savings, len(value))
	}
}
