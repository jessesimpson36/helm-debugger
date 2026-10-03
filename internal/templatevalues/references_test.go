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

func TestParseReferences(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []Reference
	}{
		{
			name: "dot reference",
			line: `{{ .Values.serviceAccount.name }}`,
			want: []Reference{{Path: "serviceAccount.name"}},
		},
		{
			name: "root reference",
			line: `{{ $.Values.serviceAccount.name }}`,
			want: []Reference{{Path: "serviceAccount.name", Root: true}},
		},
		{
			name: "root and dot references stay distinct",
			line: `{{ $.Values.image.tag }}:{{ .Values.image.tag }}`,
			want: []Reference{
				{Path: "image.tag", Root: true},
				{Path: "image.tag"},
			},
		},
		{
			name: "variable reference is not root",
			line: `{{ $x.Values.serviceAccount.name }}`,
			want: []Reference{{Path: "serviceAccount.name"}},
		},
		{
			name: "duplicate root references collapse",
			line: `{{ $.Values.image.tag }}{{ $.Values.image.tag }}`,
			want: []Reference{{Path: "image.tag", Root: true}},
		},
		{
			name: "no references",
			line: `{{ .Values }}`,
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseReferences(tt.line)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseReferences(%q) = %#v, want %#v", tt.line, got, tt.want)
			}
		})
	}
}

func TestReferenceExpr(t *testing.T) {
	if got := (Reference{Path: "image.tag"}).Expr(); got != ".Values.image.tag" {
		t.Fatalf("dot Expr() = %q", got)
	}
	if got := (Reference{Path: "image.tag", Root: true}).Expr(); got != "$.Values.image.tag" {
		t.Fatalf("root Expr() = %q", got)
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
