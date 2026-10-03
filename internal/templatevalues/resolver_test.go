package templatevalues

import (
	"reflect"
	"testing"

	"github.com/go-delve/delve/service/api"
)

func iface(v api.Variable) api.Variable {
	return api.Variable{Kind: reflect.Interface, Children: []api.Variable{v}}
}

func mapVar(pairs ...api.Variable) api.Variable {
	return api.Variable{Kind: reflect.Map, Type: "map[string]interface {}", Children: pairs}
}

func entry(key string, value api.Variable) []api.Variable {
	return []api.Variable{
		{Kind: reflect.String, Value: key},
		value,
	}
}

func stringVar(s string) api.Variable {
	return api.Variable{Kind: reflect.String, Type: "string", Value: s}
}

func TestUnwrapFollowsInterface(t *testing.T) {
	inner := stringVar("hello")
	got := unwrap(iface(iface(inner)))
	if got.Kind != reflect.String || got.Value != "hello" {
		t.Fatalf("unwrap = %+v, want the inner string", got)
	}
}

func TestUnwrapFollowsPointer(t *testing.T) {
	inner := stringVar("hello")
	ptr := api.Variable{Kind: reflect.Pointer, Children: []api.Variable{inner}}
	got := unwrap(ptr)
	if got.Kind != reflect.String || got.Value != "hello" {
		t.Fatalf("unwrap = %+v, want the pointed-to string", got)
	}
}

func TestMapLookupFindsValue(t *testing.T) {
	serviceAccount := mapVar(entry("name", stringVar(""))...)
	values := mapVar(entry("serviceAccount", serviceAccount)...)
	root := mapVar(entry("Values", values)...)

	got, ok := mapLookup(root, "Values")
	if !ok {
		t.Fatal("expected to find Values")
	}
	sa, ok := mapLookup(unwrap(got), "serviceAccount")
	if !ok {
		t.Fatal("expected to find serviceAccount")
	}
	name, ok := mapLookup(unwrap(sa), "name")
	if !ok {
		t.Fatal("expected to find name")
	}
	if got := Format(unwrap(name)); got != `""` {
		t.Fatalf("Format(unwrap(name)) = %q, want %q", got, `""`)
	}
}

func TestMapLookupMissing(t *testing.T) {
	m := mapVar(entry("present", stringVar("x"))...)
	if _, ok := mapLookup(m, "absent"); ok {
		t.Fatal("did not expect to find an absent key")
	}
}

func TestLookupPath(t *testing.T) {
	serviceAccount := mapVar(entry("name", stringVar("release-name-test"))...)
	values := mapVar(entry("serviceAccount", iface(serviceAccount))...)

	got, ok := lookupPath(unwrap(iface(values)), "serviceAccount.name")
	if !ok {
		t.Fatal("expected to resolve serviceAccount.name")
	}
	if got.Value != "release-name-test" {
		t.Fatalf("lookupPath value = %q, want %q", got.Value, "release-name-test")
	}

	if _, ok := lookupPath(values, "serviceAccount.missing"); ok {
		t.Fatal("did not expect to resolve a missing path")
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		name string
		v    api.Variable
		want string
	}{
		{"empty string is quoted", stringVar(""), `""`},
		{"string is quoted", stringVar("nginx"), `"nginx"`},
		{"bool", api.Variable{Kind: reflect.Bool, Value: "true"}, "true"},
		{"int", api.Variable{Kind: reflect.Int, Value: "3"}, "3"},
		{"nil interface", api.Variable{Kind: reflect.Interface}, "nil"},
		{"composite falls back to type", api.Variable{Kind: reflect.Map, Type: "map[string]interface {}"}, "<map[string]interface {}>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Format(tt.v); got != tt.want {
				t.Fatalf("Format(%+v) = %q, want %q", tt.v, got, tt.want)
			}
		})
	}
}
