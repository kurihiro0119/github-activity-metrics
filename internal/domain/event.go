package domain

import "time"

// EventType represents the type of GitHub event
type EventType string

const (
	EventTypeCommit      EventType = "commit"
	EventTypePullRequest EventType = "pull_request"
	EventTypeDeploy      EventType = "deploy"
)

// Event represents a raw GitHub event
type Event struct {
	ID        string
	Type      EventType
	Org       string
	Repo      string
	Member    string
	Timestamp time.Time
	Data      map[string]interface{}
	CreatedAt time.Time
}

// CommitEvent represents a commit event with additional details
type CommitEvent struct {
	ID           string
	Org          string
	Repo         string
	Member       string
	Timestamp    time.Time
	Sha          string
	Message      string
	Additions    int
	Deletions    int
	FilesChanged int
	CreatedAt    time.Time
}

// ToEvent converts CommitEvent to Event
func (c *CommitEvent) ToEvent() *Event {
	return &Event{
		ID:        c.ID,
		Type:      EventTypeCommit,
		Org:       c.Org,
		Repo:      c.Repo,
		Member:    c.Member,
		Timestamp: c.Timestamp,
		Data: map[string]interface{}{
			"sha":           c.Sha,
			"message":       c.Message,
			"additions":     c.Additions,
			"deletions":     c.Deletions,
			"files_changed": c.FilesChanged,
		},
		CreatedAt: c.CreatedAt,
	}
}

// PullRequestEvent represents a pull request event with additional details
type PullRequestEvent struct {
	ID        string
	Org       string
	Repo      string
	Member    string
	Timestamp time.Time
	Number    int
	State     string // open, closed, merged
	Title     string
	MergedAt  *time.Time
	CreatedAt time.Time
}

// ToEvent converts PullRequestEvent to Event
func (p *PullRequestEvent) ToEvent() *Event {
	data := map[string]interface{}{
		"number": p.Number,
		"state":  p.State,
		"title":  p.Title,
	}
	if p.MergedAt != nil {
		data["merged_at"] = p.MergedAt.Format(time.RFC3339)
	}
	return &Event{
		ID:        p.ID,
		Type:      EventTypePullRequest,
		Org:       p.Org,
		Repo:      p.Repo,
		Member:    p.Member,
		Timestamp: p.Timestamp,
		Data:      data,
		CreatedAt: p.CreatedAt,
	}
}

// DeployEvent represents a deployment event with additional details
type DeployEvent struct {
	ID            string
	Org           string
	Repo          string
	Member        string
	Timestamp     time.Time
	Environment   string
	Status        string
	WorkflowRunID string
	CreatedAt     time.Time
}

// ToEvent converts DeployEvent to Event
func (d *DeployEvent) ToEvent() *Event {
	return &Event{
		ID:        d.ID,
		Type:      EventTypeDeploy,
		Org:       d.Org,
		Repo:      d.Repo,
		Member:    d.Member,
		Timestamp: d.Timestamp,
		Data: map[string]interface{}{
			"environment":     d.Environment,
			"status":          d.Status,
			"workflow_run_id": d.WorkflowRunID,
		},
		CreatedAt: d.CreatedAt,
	}
}
