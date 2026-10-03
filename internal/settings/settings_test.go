package settings

import (
	"reflect"
	"testing"
)

func TestSplitArgs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty", "", nil},
		{"simple", "--show-only templates/deployment.yaml", []string{"--show-only", "templates/deployment.yaml"}},
		{"double quoted", `--set image.tag="1 2"`, []string{"--set", "image.tag=1 2"}},
		{"single quoted", `--set foo='bar baz'`, []string{"--set", "foo=bar baz"}},
		{"escaped space", `--set foo=bar\ baz`, []string{"--set", "foo=bar baz"}},
		{"extra whitespace", "  a\tb\nc ", []string{"a", "b", "c"}},
		{"empty quoted", `--set foo=""`, []string{"--set", "foo="}},
		{"adjacent quotes", `ab"cd"ef`, []string{"abcdef"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitArgs(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("SplitArgs(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitCommaDelimited(t *testing.T) {
	got := SplitCommaDelimited(" a, b , ,c ")
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitCommaDelimited = %#v, want %#v", got, want)
	}
	if SplitCommaDelimited("") != nil {
		t.Fatal("expected nil for empty input")
	}
}

func TestValidate(t *testing.T) {
	cfg := &Settings{ChartName: "test", CompiledHelmPath: "helm"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := (&Settings{CompiledHelmPath: "helm"}).Validate(); err == nil {
		t.Fatal("expected error when chart is missing")
	}
	if err := (&Settings{ChartName: "test"}).Validate(); err == nil {
		t.Fatal("expected error when helm path is missing")
	}
}

func TestCloneIsIndependent(t *testing.T) {
	cfg := &Settings{CommandArgs: []string{"a"}, ValuesQuery: []string{"b"}}
	clone := cfg.Clone()
	clone.CommandArgs[0] = "changed"
	clone.ValuesQuery[0] = "changed"
	if cfg.CommandArgs[0] != "a" || cfg.ValuesQuery[0] != "b" {
		t.Fatal("clone shares backing arrays with the original")
	}
}
