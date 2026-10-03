package debugger

import "testing"

func TestExprFor(t *testing.T) {
	tests := []struct {
		name   string
		idents []string
		want   string
	}{
		{"dot chain", []string{"Values", "serviceAccount", "name"}, ".Values.serviceAccount.name"},
		{"root chain", []string{"$", "Values", "image", "tag"}, "$.Values.image.tag"},
		{"empty", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exprFor(tt.idents); got != tt.want {
				t.Fatalf("exprFor(%v) = %q, want %q", tt.idents, got, tt.want)
			}
		})
	}
}

func TestLineAt(t *testing.T) {
	text := "line1\nline2\nline3\n"
	tests := []struct {
		pos  int
		want int
	}{
		{0, 1},
		{5, 1},  // still on line 1
		{6, 2},  // first byte of line 2
		{12, 3}, // first byte of line 3
		{99, 4}, // clamps past the end
		{-1, 1}, // clamps below zero
	}
	for _, tt := range tests {
		if got := lineAt(text, tt.pos); got != tt.want {
			t.Fatalf("lineAt(text, %d) = %d, want %d", tt.pos, got, tt.want)
		}
	}
}
