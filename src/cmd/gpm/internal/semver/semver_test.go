package semver

import "testing"

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"0.9.0", "1.0.0", -1},
		{"1.10.0", "1.9.9", 1},
		{"1.0.0-beta.1", "1.0.0", -1},
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestPickBestExact(t *testing.T) {
	vs := []string{"1.0.0", "1.2.0", "2.0.0"}
	if got, ok := PickBest(vs, "1.2.0", ""); !ok || got != "1.2.0" {
		t.Errorf("exact match failed: got %q ok=%v", got, ok)
	}
}

func TestPickBestCaret(t *testing.T) {
	vs := []string{"0.9.9", "1.0.0", "1.2.7", "1.5.0", "2.0.0", "2.1.0"}
	got, ok := PickBest(vs, "^1.2.7", "")
	if !ok || got != "1.5.0" {
		t.Errorf("^1.2.7 -> got %q ok=%v, want 1.5.0", got, ok)
	}
	if got, ok := PickBest(vs, "^1.2", ""); !ok || got != "1.5.0" {
		t.Errorf("^1.2 -> got %q ok=%v, want 1.5.0", got, ok)
	}
	_, ok = PickBest(vs, "^2.0", "")
	if !ok {
		t.Error("^2.0 should match")
	}
}

func TestPickBestTilde(t *testing.T) {
	vs := []string{"1.2.0", "1.2.9", "1.3.0", "2.0.0"}
	got, ok := PickBest(vs, "~1.2", "")
	if !ok || got != "1.2.9" {
		t.Errorf("~1.2 -> got %q ok=%v, want 1.2.9", got, ok)
	}
	got, ok = PickBest(vs, "~1.2.0", "")
	if !ok || got != "1.2.9" {
		t.Errorf("~1.2.0 -> got %q ok=%v, want 1.2.9", got, ok)
	}
}

func TestPickBestPartial(t *testing.T) {
	vs := []string{"1.0.0", "1.5.3", "2.2.0"}
	if got, ok := PickBest(vs, "1.x", ""); !ok || got != "1.5.3" {
		t.Errorf("1.x -> got %q ok=%v want 1.5.3", got, ok)
	}
	if got, ok := PickBest(vs, "1", ""); !ok || got != "1.5.3" {
		t.Errorf("1 -> got %q ok=%v want 1.5.3", got, ok)
	}
}

func TestPickBestRange(t *testing.T) {
	vs := []string{"1.0.0", "1.5.0", "1.9.0", "2.0.0"}
	got, ok := PickBest(vs, ">=1.2.0 <2.0.0", "")
	if !ok || got != "1.9.0" {
		t.Errorf("range -> got %q ok=%v want 1.9.0", got, ok)
	}
}

func TestPickBestStar(t *testing.T) {
	vs := []string{"0.1.0", "3.0.0", "2.0.0"}
	got, ok := PickBest(vs, "*", "")
	if !ok || got != "3.0.0" {
		t.Errorf("* -> got %q ok=%v want 3.0.0", got, ok)
	}
	got, ok = PickBest(vs, "", "")
	if !ok || got != "3.0.0" {
		t.Errorf("(empty) -> got %q ok=%v want 3.0.0", got, ok)
	}
}

func TestPickBestNoMatch(t *testing.T) {
	vs := []string{"1.0.0", "1.2.0"}
	if _, ok := PickBest(vs, "^2.0.0", ""); ok {
		t.Error("^2.0.0 should not match")
	}
}

func TestPickBestPrereleaseExcluded(t *testing.T) {
	vs := []string{"1.0.0-beta.1", "1.0.0"}
	got, ok := PickBest(vs, "^1.0.0", "")
	if !ok || got != "1.0.0" {
		t.Errorf("stable preferred: got %q ok=%v", got, ok)
	}
}
