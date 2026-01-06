package domain

import "time"

// CollectionBatch represents a batch collection job
type CollectionBatch struct {
	ID        string
	Mode      string // "organization" or "user"
	Owner     string // organization name or user name
	StartDate time.Time
	EndDate   time.Time
	Status    string // "in_progress", "completed", "failed"
	CreatedAt time.Time
	UpdatedAt time.Time
}

