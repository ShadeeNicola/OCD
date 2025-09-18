package hf

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// DiffItem represents a single property version change
type DiffItem struct {
	Property string `json:"property"`
	Current  string `json:"current"`
	Proposed string `json:"proposed"`
	Change   string `json:"change"` // upgrade | downgrade | same | new | missing
}

// ExtractVersions finds version values for the provided property keys in a pom.xml content
func ExtractVersions(pomXML []byte, keys []string) map[string]string {
	current := map[string]string{}
	for _, key := range keys {
		re := regexp.MustCompile(fmt.Sprintf(`<%s>\s*([^<\s]+)\s*</%s>`, regexp.QuoteMeta(key), regexp.QuoteMeta(key)))
		m := re.FindSubmatch(pomXML)
		if len(m) == 2 {
			current[key] = string(m[1])
		}
	}
	return current
}

// BuildDiff compares current vs proposed maps and returns a list of diffs
func BuildDiff(current, proposed map[string]string) []DiffItem {
	seen := map[string]struct{}{}
	var items []DiffItem
	for prop, newVal := range proposed {
		oldVal := current[prop]
		change := compareChange(oldVal, newVal)
		items = append(items, DiffItem{Property: prop, Current: oldVal, Proposed: newVal, Change: change})
		seen[prop] = struct{}{}
	}
	// Include properties present in current but not proposed
	for prop, oldVal := range current {
		if _, ok := seen[prop]; !ok {
			items = append(items, DiffItem{Property: prop, Current: oldVal, Proposed: "", Change: "missing"})
		}
	}
	return items
}

func compareChange(oldVal, newVal string) string {
	if strings.TrimSpace(newVal) == "" {
		return "missing"
	}
	if oldVal == "" {
		return "new"
	}
	cmp := compareVersions(oldVal, newVal)
	if cmp == 0 {
		return "same"
	}
	if cmp < 0 {
		return "upgrade"
	}
	return "downgrade"
}

// compareVersions compares version strings by numeric tokens first then text
// Returns -1 if a<b, 0 if equal, 1 if a>b
func compareVersions(a, b string) int {
	// Normalize
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == b {
		return 0
	}

	// Split into mixed tokens of digits and non-digits
	tokenize := func(s string) []string {
		re := regexp.MustCompile(`(\d+|[^\d]+)`)
		return re.FindAllString(s, -1)
	}

	ta := tokenize(a)
	bv := tokenize(b)
	max := len(ta)
	if len(bv) > max {
		max = len(bv)
	}
	for i := 0; i < max; i++ {
		var sa, sb string
		if i < len(ta) {
			sa = ta[i]
		}
		if i < len(bv) {
			sb = bv[i]
		}
		if sa == sb {
			continue
		}
		// Try numeric comparison
		na, ea := strconv.Atoi(sa)
		nb, eb := strconv.Atoi(sb)
		if ea == nil && eb == nil {
			if na < nb {
				return -1
			}
			if na > nb {
				return 1
			}
			continue
		}
		// Fallback string compare
		if sa < sb {
			return -1
		}
		if sa > sb {
			return 1
		}
	}
	return 0
}

// UpdatePOMVersions replaces property values in pom.xml and returns updated content
func UpdatePOMVersions(pomXML []byte, versions map[string]string) ([]byte, error) {
	out := pomXML
	for prop, newVal := range versions {
		if strings.TrimSpace(newVal) == "" {
			continue
		}
		// Preserve inner whitespace and avoid corrupting tags by capturing value only
		pattern := fmt.Sprintf(`(<\s*%s\s*>\s*)([^<]*?)(\s*</\s*%s\s*>)`, regexp.QuoteMeta(prop), regexp.QuoteMeta(prop))
		re := regexp.MustCompile(pattern)
		// Use ${1} and ${3} to avoid ambiguity with digits in newVal (e.g., leading '10' being parsed as $10)
		replacement := fmt.Sprintf(`${1}%s${3}`, newVal)
		out = re.ReplaceAll(out, []byte(replacement))
	}
	return out, nil
}

// PrettyDiffText returns a simple textual summary for UI/debug
func PrettyDiffText(items []DiffItem) string {
	var buf bytes.Buffer
	for _, it := range items {
		buf.WriteString(fmt.Sprintf("%s: %s -> %s (%s)\n", it.Property, it.Current, it.Proposed, it.Change))
	}
	return buf.String()
}
