package secrets

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"
)

// Mask is what a secret value is replaced with in logs and outputs.
const Mask = "••••••"

// minMaskLen avoids mangling unrelated text: masking a 2-character "secret"
// would rewrite half of every log line.
const minMaskLen = 4

// Masker replaces known secret values in text.
type Masker struct {
	r *strings.Replacer
}

func NewMasker(values []string) *Masker {
	var vals []string
	for _, v := range values {
		vals = append(vals, v)
		// Inside stored JSON (configs, outputs) the value appears escaped.
		if esc := jsonEscaped(v); esc != v {
			vals = append(vals, esc)
		}
	}
	// Longer values first, so a secret containing another is masked whole.
	slices.SortFunc(vals, func(a, b string) int { return cmp.Compare(len(b), len(a)) })
	var pairs []string
	for _, v := range vals {
		if len(v) >= minMaskLen {
			pairs = append(pairs, v, Mask)
		}
	}
	if len(pairs) == 0 {
		return &Masker{}
	}
	return &Masker{r: strings.NewReplacer(pairs...)}
}

func (m *Masker) Mask(s string) string {
	if m == nil || m.r == nil {
		return s
	}
	return m.r.Replace(s)
}

func jsonEscaped(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}
