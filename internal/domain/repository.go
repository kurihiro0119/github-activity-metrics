package domain

import "time"

// Repository represents a GitHub repository
type Repository struct {
	Org          string
	Name         string
	FullName     string
	IsPrivate    bool
	LastSyncedAt *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Member represents a GitHub organization member
type Member struct {
	Org          string
	Username     string
	DisplayName  string
	LastSyncedAt *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
