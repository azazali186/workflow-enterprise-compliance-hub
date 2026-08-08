package models

// All returns every model so migrations can register them in one place.
func All() []any {
	return []any{
		&Regulation{},
		&Compliance{},
		&Audit{},
		&Checklist{},
		&Alert{},
		&Report{},
		&Violation{},
		&CorrectiveAction{},
		&Deadline{},
		&AuditLog{},
		&Permission{},
		&Role{},
		&User{},
		&OutboxEvent{},
	}
}
