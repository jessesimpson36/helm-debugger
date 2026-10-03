package version

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGetIncludesRunningGoVersion(t *testing.T) {
	info := Get()
	if info.GoVersion == "" {
		t.Fatal("GoVersion is empty")
	}
	if info.Version == "" {
		t.Fatal("Version is empty")
	}
	if info.Platform == "" {
		t.Fatal("Platform is empty")
	}
}

func TestJSONRoundTrips(t *testing.T) {
	raw, err := Get().JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var got Info
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != Get() {
		t.Fatalf("round trip mismatch: %+v != %+v", got, Get())
	}
}

func TestStringMentionsToolchain(t *testing.T) {
	got := Get().String()
	for _, want := range []string{"helm-debugger", "go:", "helm:", "delve:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("String() = %q, missing %q", got, want)
		}
	}
}
