package report

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jessesimpson36/helm-debugger/internal/executionflow"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"github.com/jessesimpson36/helm-debugger/internal/settings"
)

// maxLocateSites caps how many source locations the compact report returns so an
// unfiltered query cannot flood the caller.
const maxLocateSites = 200

// Site is one template or helper source location that participates in a matched
// execution flow. Helper is the helper function name when the location is
// inside a helper, and empty for a location in a top-level template.
type Site struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Source string `json:"source,omitempty"`
	Helper string `json:"helper,omitempty"`
	// Values holds the .Values.* references on this source line mapped to the
	// values Helm rendered with. It is only populated when value resolution was
	// enabled; the map keys omit the leading ".Values.".
	Values map[string]string `json:"values,omitempty"`
}

// Located is the compact, machine-readable answer to a debug query. Unlike the
// full report it omits the rendered write buffers, which makes the
// "where is this value read / what writes this output" question cheap to ask
// before editing a template.
type Located struct {
	Sections       []Section
	Sites          []Site
	RelevantValues []string
	Truncated      bool
}

// Locate applies cfg's queries to flows and collects the source locations and
// referenced values behind the matches. For a values query the actual read
// sites are reported first and unrelated helper frames are omitted, so the
// "which file:line do I edit" answer stays narrow. Sites are deduplicated in
// flow order.
func Locate(flows []*executionflow.ExecutionFlow, cfg *settings.Settings) Located {
	sections := Sections(flows, cfg)
	located := Located{Sections: sections}

	siteSeen := map[string]struct{}{}
	valueSeen := map[string]struct{}{}
	addSite := func(site Site) {
		if site.File == "" {
			return
		}
		key := siteKey(site)
		if _, ok := siteSeen[key]; ok {
			return
		}
		if len(located.Sites) >= maxLocateSites {
			located.Truncated = true
			return
		}
		siteSeen[key] = struct{}{}
		located.Sites = append(located.Sites, site)
	}

	// A values query is answered by the lines that actually read the option, so
	// those go first; only the queried option's reads are kept, and the
	// enclosing helper chain is not dumped.
	valuesQuery := cfg != nil && len(cfg.ValuesQuery) > 0
	for _, section := range sections {
		for _, flow := range section.Flows {
			if flow == nil {
				continue
			}
			for _, valRef := range flow.ValuesReference {
				matched := !valuesQuery || valueNameMatches(valRef.ValuesName, cfg.ValuesQuery)
				if !matched {
					continue
				}
				if valRef.ValuesName != "" {
					valueSeen[valRef.ValuesName] = struct{}{}
				}
				addSite(siteForUnit(valRef.ExecutionUnit))
			}
		}
	}
	for _, section := range sections {
		for _, flow := range section.Flows {
			if flow == nil {
				continue
			}
			addSite(siteForUnit(flow.Template))
			if valuesQuery {
				continue
			}
			for _, helper := range flow.Helpers {
				addSite(siteForUnit(helper))
			}
		}
	}

	located.RelevantValues = sortedStringSet(valueSeen)
	return located
}

// valueNameMatches mirrors query.QueryValuesReference prefix matching so locate
// only reports the reads of the options that were actually queried.
func valueNameMatches(name string, queries []string) bool {
	for _, query := range queries {
		if strings.HasPrefix(name, query) {
			return true
		}
	}
	return false
}

func siteForUnit(unit *frame.ExecutionUnit) Site {
	if unit == nil {
		return Site{}
	}
	helper := ""
	if unit.FunctionName != unit.FileName {
		helper = unit.FunctionName
	}
	return Site{
		File:   unit.FileName,
		Line:   unit.LineNumber,
		Source: unit.LineContent,
		Helper: helper,
		Values: unit.ResolvedValues,
	}
}

// siteKey identifies a source location for deduplication. It deliberately
// excludes Site.Values, which is not comparable, so the first capture of a
// location wins.
func siteKey(s Site) string {
	return s.File + ":" + strconv.Itoa(s.Line) + ":" + s.Helper
}

func sortedStringSet(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// sortedValueKeys returns the sorted keys of a resolved-values map.
func sortedValueKeys(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// LocateText renders the compact located sites for humans. suggestions, when
// non-empty, are appended as "did you mean" hints, used when nothing matched.
func LocateText(located Located, suggestions []string) string {
	var b strings.Builder
	fmt.Fprintln(&b, "================= LOCATE =================")
	fmt.Fprintf(&b, "Source sites (%d):\n", len(located.Sites))
	for _, site := range located.Sites {
		if site.Helper != "" {
			fmt.Fprintf(&b, "  %s:%d  in %s\n", site.File, site.Line, site.Helper)
		} else {
			fmt.Fprintf(&b, "  %s:%d\n", site.File, site.Line)
		}
		if site.Source != "" {
			fmt.Fprintf(&b, "      %s\n", strings.TrimSpace(site.Source))
		}
		for _, name := range sortedValueKeys(site.Values) {
			fmt.Fprintf(&b, "      .Values.%s = %s\n", name, site.Values[name])
		}
	}
	if located.Truncated {
		fmt.Fprintf(&b, "  ... (truncated at %d sites; narrow the query)\n", maxLocateSites)
	}
	fmt.Fprintln(&b, "Relevant values:")
	if len(located.RelevantValues) == 0 {
		fmt.Fprintln(&b, "  (none)")
	}
	for _, name := range located.RelevantValues {
		fmt.Fprintf(&b, "  - %s\n", name)
	}
	if len(located.Sites) == 0 {
		fmt.Fprintln(&b, "No source sites matched the query.")
	}
	if len(suggestions) > 0 {
		fmt.Fprintln(&b, "Did you mean:")
		for _, suggestion := range suggestions {
			fmt.Fprintf(&b, "  - %s\n", suggestion)
		}
	}
	return b.String()
}
