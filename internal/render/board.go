package render

import (
	"strings"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/nmizern/tgira/internal/i18n"
)

// Telegram caps a message at 4096 characters; leave room for the closing line.
const (
	boardLimit      = 4096
	boardTailBudget = 64
	boardTitleRunes = 60
)

// BoardText renders the pinned board: everything still open, doing first.
func BoardText(b domain.Board, tasks []domain.Task, u Users, l i18n.Strings) string {
	groups := []struct {
		status domain.Status
		tasks  []domain.Task
	}{
		{status: domain.StatusDoing},
		{status: domain.StatusTodo},
	}

	open := 0
	for _, t := range tasks {
		for i := range groups {
			if t.Status == groups[i].status {
				groups[i].tasks = append(groups[i].tasks, t)
				open++
			}
		}
	}

	head := "📋 <b>" + esc(b.Title) + "</b>"
	if open == 0 {
		return head + "\n\n" + esc(l.T(i18n.KeyBoardEmpty))
	}
	head += " · " + esc(l.T(i18n.KeyBoardOpen, open))

	var (
		body    []string
		used    = len([]rune(head))
		skipped int
	)
	for _, g := range groups {
		if len(g.tasks) == 0 {
			continue
		}

		section := "\n<b>" + esc(l.Status(g.status)) + "</b>"
		if skipped == 0 && used+len([]rune(section))+boardTailBudget <= boardLimit {
			body = append(body, section)
			used += len([]rune(section))
		}

		for _, t := range g.tasks {
			line := boardLine(b, t, u)
			if skipped > 0 || used+len([]rune(line))+boardTailBudget > boardLimit {
				skipped++
				continue
			}
			body = append(body, line)
			used += len([]rune(line))
		}
	}

	out := head + "\n" + strings.Join(body, "\n")
	if skipped > 0 {
		out += "\n" + esc(l.T(i18n.KeyBoardMore, skipped))
	}
	return out
}

func boardLine(b domain.Board, t domain.Task, u Users) string {
	who := esc(assignee(t, u))
	return marker(t) + " " + key(b, t) + " · " + esc(shorten(t.Title)) + metaSuffix(who)
}

func metaSuffix(who string) string {
	if who == "" {
		return ""
	}
	return " · " + who
}

func shorten(title string) string {
	runes := []rune(title)
	if len(runes) <= boardTitleRunes {
		return title
	}
	return strings.TrimSpace(string(runes[:boardTitleRunes-1])) + "…"
}
