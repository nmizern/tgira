package parse

import (
	"sort"
	"unicode/utf16"
)

// Span is a region of text measured in UTF-16 code units, the way Telegram
// counts message entity offsets.
type Span struct {
	Offset int
	Length int
}

func (s Span) end() int { return s.Offset + s.Length }

// text16 is a string indexed the way Telegram indexes it.
type text16 struct {
	units []uint16
}

func newText16(s string) text16 {
	return text16{units: utf16.Encode([]rune(s))}
}

func (t text16) len() int { return len(t.units) }

func (t text16) String() string { return string(utf16.Decode(t.units)) }

// slice returns the text a span covers, clamped to what actually exists.
func (t text16) slice(s Span) string {
	from, to := clamp(s.Offset, len(t.units)), clamp(s.end(), len(t.units))
	if from >= to {
		return ""
	}
	return string(utf16.Decode(t.units[from:to]))
}

// cut removes every span and returns what is left.
func (t text16) cut(spans []Span) string {
	if len(spans) == 0 {
		return t.String()
	}

	merged := mergeSpans(spans, len(t.units))
	out := make([]uint16, 0, len(t.units))
	at := 0
	for _, s := range merged {
		if s.Offset > at {
			out = append(out, t.units[at:s.Offset]...)
		}
		if s.end() > at {
			at = s.end()
		}
	}
	if at < len(t.units) {
		out = append(out, t.units[at:]...)
	}
	return string(utf16.Decode(out))
}

func mergeSpans(spans []Span, limit int) []Span {
	sorted := make([]Span, 0, len(spans))
	for _, s := range spans {
		from, to := clamp(s.Offset, limit), clamp(s.end(), limit)
		if from < to {
			sorted = append(sorted, Span{Offset: from, Length: to - from})
		}
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Offset < sorted[j].Offset })

	var out []Span
	for _, s := range sorted {
		if n := len(out); n > 0 && s.Offset <= out[n-1].end() {
			if end := max(out[n-1].end(), s.end()); end > out[n-1].end() {
				out[n-1].Length = end - out[n-1].Offset
			}
			continue
		}
		out = append(out, s)
	}
	return out
}

func clamp(v, limit int) int {
	if v < 0 {
		return 0
	}
	if v > limit {
		return limit
	}
	return v
}
