// Package parse turns a chat message into the draft of a task.
// It depends on nothing but the domain types.
package parse

import (
	"regexp"
	"sort"
	"strings"

	"github.com/nmizern/tgira/internal/domain"
)

// Telegram entity types this package cares about.
const (
	EntityMention     = "mention"
	EntityTextMention = "text_mention"
	EntityHashtag     = "hashtag"
)

// Priority parsing modes.
const (
	PriorityEdges    = "edges"
	PriorityAnywhere = "anywhere"
)

// Entity is a Telegram message entity, offsets in UTF-16 code units.
type Entity struct {
	Type   string
	Offset int
	Length int
	UserID int64 // set for text_mention only
}

// Options tunes how forgiving the parser is.
type Options struct {
	PriorityPosition string
}

// Draft is what a message says a task should be.
type Draft struct {
	Title            string
	Description      string
	Priority         domain.Priority
	Tags             []string
	AssigneeUsername string
	AssigneeID       int64
}

var (
	mentionRe = regexp.MustCompile(`(^|\s)@([A-Za-z0-9_]{1,32})`)
	hashtagRe = regexp.MustCompile(`(^|\s)#([\p{L}\p{N}_]+)`)
)

type marker struct {
	span   Span
	kind   string
	userID int64
}

// Parse extracts the assignee, priority and tags, and treats what is left as
// the task itself. Entities are trusted when Telegram supplies them; the
// regular expressions cover text that arrives without any, such as commands.
func Parse(text string, entities []Entity, opts Options) Draft {
	body := newText16(text)
	markers := collectMarkers(body, entities)

	var (
		draft Draft
		cuts  []Span
		seen  = map[string]bool{}
	)
	for _, m := range markers {
		word := body.slice(m.span)
		switch m.kind {
		case EntityHashtag:
			tag := strings.ToLower(strings.TrimPrefix(word, "#"))
			if tag != "" && !seen[tag] {
				seen[tag] = true
				draft.Tags = append(draft.Tags, tag)
			}
			cuts = append(cuts, m.span)

		case EntityMention, EntityTextMention:
			// Later mentions are part of the text; only the first one assigns.
			if draft.AssigneeUsername != "" || draft.AssigneeID != 0 {
				continue
			}
			if m.kind == EntityTextMention {
				draft.AssigneeID = m.userID
			} else {
				draft.AssigneeUsername = strings.TrimPrefix(word, "@")
			}
			cuts = append(cuts, m.span)
		}
	}

	lines := strings.Split(body.cut(cuts), "\n")
	draft.Priority, draft.Title = takePriority(lines[0], opts)
	draft.Description = strings.TrimSpace(strings.Join(lines[1:], "\n"))
	return draft
}

func collectMarkers(body text16, entities []Entity) []marker {
	markers := make([]marker, 0, len(entities))
	at := map[int]bool{}

	for _, e := range entities {
		switch e.Type {
		case EntityMention, EntityTextMention, EntityHashtag:
			markers = append(markers, marker{
				span:   Span{Offset: e.Offset, Length: e.Length},
				kind:   e.Type,
				userID: e.UserID,
			})
			at[e.Offset] = true
		}
	}

	text := body.String()
	for kind, re := range map[string]*regexp.Regexp{EntityMention: mentionRe, EntityHashtag: hashtagRe} {
		for _, span := range scan(text, re) {
			if at[span.Offset] {
				continue
			}
			at[span.Offset] = true
			markers = append(markers, marker{span: span, kind: kind})
		}
	}

	sort.Slice(markers, func(i, j int) bool { return markers[i].span.Offset < markers[j].span.Offset })
	return markers
}

// scan finds every match of the second group and reports it in UTF-16 offsets.
func scan(text string, re *regexp.Regexp) []Span {
	var out []Span
	for _, loc := range re.FindAllStringSubmatchIndex(text, -1) {
		// loc[2:4] is the leading boundary, loc[4:6] the word after @ or #
		start := loc[4] - 1
		offset := newText16(text[:start]).len()
		length := newText16(text[start:loc[5]]).len()
		out = append(out, Span{Offset: offset, Length: length})
	}
	return out
}

// takePriority pulls a lone 1, 2 or 3 out of the line and returns the rest,
// with whitespace squeezed out either way.
func takePriority(line string, opts Options) (domain.Priority, string) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return domain.PriorityNone, ""
	}

	found := -1
	if opts.PriorityPosition == PriorityAnywhere {
		for i, f := range fields {
			if isPriority(f) {
				found = i
				break
			}
		}
	} else {
		switch {
		case isPriority(fields[0]):
			found = 0
		case isPriority(fields[len(fields)-1]):
			found = len(fields) - 1
		}
	}

	if found < 0 {
		return domain.PriorityNone, strings.Join(fields, " ")
	}

	priority := domain.Priority(fields[found][0] - '0')
	rest := append(append([]string{}, fields[:found]...), fields[found+1:]...)
	return priority, strings.Join(rest, " ")
}

func isPriority(field string) bool {
	return field == "1" || field == "2" || field == "3"
}
