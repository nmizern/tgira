package domain

// CanChangeStatus reports whether an actor may move a task. A task nobody owns
// is up for grabs; an owned one belongs to its author and its assignee.
func CanChangeStatus(t Task, actorID int64) bool {
	if t.AssigneeID == 0 {
		return true
	}
	return actorID == t.AuthorID || actorID == t.AssigneeID
}

// CanEdit reports whether an actor may rewrite a task's text or fields.
func CanEdit(t Task, actorID int64) bool {
	return CanChangeStatus(t, actorID)
}

// CanDelete reports whether an actor may remove a task. Only the author may,
// because a deleted card disappears for everyone.
func CanDelete(t Task, actorID int64) bool {
	return actorID == t.AuthorID
}
