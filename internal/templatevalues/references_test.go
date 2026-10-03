package templatevalues

import (
	"reflect"
	"testing"
)

func TestContains(t *testing.T) {
	if Contains("no values here") {
		t.Fatal("expected false for a line without .Values")
	}
	if !Contains("{{ .Values.foo }}") {
		t.Fatal("expected true for a line referencing .Values")
	}
}

func TestReferences(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
	}{
		{
			name: "two references in one line",
			line: `  image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"`,
			want: []string{"image.repository", "image.tag"},
		},
		{
			name: "reference without surrounding spaces",
			line: `{{.Values.serviceAccount.name}}`,
			want: []string{"serviceAccount.name"},
		},
		{
			name: "duplicate references collapse",
			line: `{{ .Values.image.tag }}{{ .Values.image.tag }}`,
			want: []string{"image.tag"},
		},
		{
			name: "nested argument pipeline",
			line: `{{- default (include "test.fullname" .) .Values.serviceAccount.name }}`,
			want: []string{"serviceAccount.name"},
		},
		{
			name: "no references",
			line: `{{ printf "x" }}`,
			want: nil,
		},
		{
			name: "bare .Values is ignored",
			line: `{{ .Values }}`,
			want: nil,
		},
		{
			name: "dollar root reference is still found",
			line: `{{ $.Values.global.name }}`,
			want: []string{"global.name"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := References(tt.line)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("References(%q) = %#v, want %#v", tt.line, got, tt.want)
			}
		})
	}
}
