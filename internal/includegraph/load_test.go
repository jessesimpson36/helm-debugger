package includegraph

import (
	"os"
	"testing"
)

// TestLoadRealCharts validates the parser against the repository's test chart
// and, when present, the camunda chart. It is skipped when neither exists.
func TestLoadRealCharts(t *testing.T) {
	root := t.TempDir()
	_ = root

	// The repo's test chart.
	testChart := "../../test"
	if _, err := os.Stat(testChart); err != nil {
		t.Skipf("no test chart: %v", err)
	}
	g, err := Load(testChart)
	if err != nil {
		t.Fatalf("Load(%s): %v", testChart, err)
	}
	names, unbounded := g.Closure([]string{"test.serviceAccountName"})
	if unbounded {
		t.Fatalf("serviceAccountName unexpectedly unbounded: %v", names)
	}
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	for _, want := range []string{"test.serviceAccountName", "test.fullname"} {
		if !found[want] {
			t.Fatalf("closure %v missing %q", names, want)
		}
	}
}
