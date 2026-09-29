package domain

// CanChangeStatus reports whether an actor may move a task. A task nobody owns
// is up for grabs; an owned one belongs to its author and its assignee.
func CanChangeStatus(t Task, actorID int64) bool {
	if !t.Assigned() {
		return true
	}
	return actorID == t.AuthorID || actorID == t.AssigneeID
}

// CanEdit reports whether an actor may rewrite a task's text or fields.
func CanEdit(t Task, actorID int64) bool {
	return CanChangeStatus(t, actorID)
}

// Removing a task is deliberately open to everyone on the board: a message
// that should never have become a ticket is the team's mess, not its author's.
// Nothing is lost either way — the row and its history stay behind.
