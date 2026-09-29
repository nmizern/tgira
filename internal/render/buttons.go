package render

import (
	"strconv"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/nmizern/tgira/internal/i18n"
)

// Callback names the card's buttons answer to.
const (
	StatusAction = "status"
	DeleteAction = "remove"
)

// Button is one inline keyboard button, ready for the Telegram layer to wrap.
type Button struct {
	Text   string
	Unique string
	Data   string
}

// Buttons are the moves that make sense from a task's current status.
func Buttons(t domain.Task, l i18n.Strings) []Button {
	move := func(key string, to domain.Status) Button {
		return Button{
			Text:   l.T(key),
			Unique: StatusAction,
			Data:   strconv.FormatInt(t.ID, 10) + ":" + string(to),
		}
	}

	remove := Button{
		Text:   l.T(i18n.KeyBtnDelete),
		Unique: DeleteAction,
		Data:   strconv.FormatInt(t.ID, 10),
	}

	switch t.Status {
	case domain.StatusTodo:
		return []Button{move(i18n.KeyBtnTake, domain.StatusDoing), move(i18n.KeyBtnDone, domain.StatusDone), remove}
	case domain.StatusDoing:
		return []Button{move(i18n.KeyBtnDone, domain.StatusDone), move(i18n.KeyBtnTodo, domain.StatusTodo), remove}
	default:
		return []Button{move(i18n.KeyBtnReopen, domain.StatusTodo), remove}
	}
}
