package parse

import (
	"strings"
	"testing"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/stretchr/testify/require"
)

// ent builds an entity the way Telegram would send it, in UTF-16 offsets.
func ent(text, sub, kind string, userID int64) Entity {
	idx := strings.Index(text, sub)
	if idx < 0 {
		panic("substring not found: " + sub)
	}
	return Entity{
		Type:   kind,
		Offset: newText16(text[:idx]).len(),
		Length: newText16(sub).len(),
		UserID: userID,
	}
}

func TestParse(t *testing.T) {
	const mention = "@ivan @petr сделать"
	const textMention = "Иван сделай это"
	const withEmoji = "1 сделать 🎉 релиз #ship"

	cases := []struct {
		name     string
		text     string
		entities []Entity
		opts     Options
		want     Draft
	}{
		{
			name: "plain text is the title",
			text: "поправить логин",
			want: Draft{Title: "поправить логин"},
		},
		{
			name: "priority, assignee and tag are lifted out",
			text: "1 @ivan поправить логин #backend",
			want: Draft{
				Title:            "поправить логин",
				Priority:         domain.PriorityHigh,
				AssigneeUsername: "ivan",
				Tags:             []string{"backend"},
			},
		},
		{
			name: "priority may close the line",
			text: "поправить логин 2",
			want: Draft{Title: "поправить логин", Priority: domain.PriorityMid},
		},
		{
			name: "a digit in the middle is part of the task",
			text: "сделать 2 кнопки в форме",
			want: Draft{Title: "сделать 2 кнопки в форме"},
		},
		{
			name: "anywhere mode does take the middle digit",
			text: "сделать 2 кнопки в форме",
			opts: Options{PriorityPosition: PriorityAnywhere},
			want: Draft{Title: "сделать кнопки в форме", Priority: domain.PriorityMid},
		},
		{
			name: "only 1 to 3 are priorities",
			text: "поправить логин 4",
			want: Draft{Title: "поправить логин 4"},
		},
		{
			name: "a bare mention leaves nothing to do",
			text: "@ivan",
			want: Draft{AssigneeUsername: "ivan"},
		},
		{
			name: "every tag is collected and lowercased",
			text: "#Backend #api починить",
			want: Draft{Title: "починить", Tags: []string{"backend", "api"}},
		},
		{
			name:     "only the first mention becomes the assignee",
			text:     mention,
			entities: []Entity{ent(mention, "@ivan", EntityMention, 0), ent(mention, "@petr", EntityMention, 0)},
			want:     Draft{Title: "@petr сделать", AssigneeUsername: "ivan"},
		},
		{
			name:     "a mention without a username carries the user id",
			text:     textMention,
			entities: []Entity{ent(textMention, "Иван", EntityTextMention, 42)},
			want:     Draft{Title: "сделай это", AssigneeID: 42},
		},
		{
			name: "the first line is the title, the rest is the description",
			text: "починить логин\n\nломается на редиректе",
			want: Draft{Title: "починить логин", Description: "ломается на редиректе"},
		},
		{
			name:     "emoji do not shift entity offsets",
			text:     withEmoji,
			entities: []Entity{ent(withEmoji, "#ship", EntityHashtag, 0)},
			want:     Draft{Title: "сделать 🎉 релиз", Priority: domain.PriorityHigh, Tags: []string{"ship"}},
		},
		{
			name: "extra whitespace is squeezed out",
			text: "   поправить    логин   ",
			want: Draft{Title: "поправить логин"},
		},
		{
			name: "the same tag twice is stored once",
			text: "#api чинить #api",
			want: Draft{Title: "чинить", Tags: []string{"api"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(tc.text, tc.entities, tc.opts)
			require.Equal(t, tc.want, got)
		})
	}
}
