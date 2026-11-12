package audit

import "context"

type Entry struct {
	ID          string         `json:"id"`
	ActorID     string         `json:"actorId"`
	Action      string         `json:"action"`
	EntityID    string         `json:"entityId"`
	CreatedAt   string         `json:"createdAt"`
	Details     map[string]any `json:"details,omitempty"`
	ActorName   string         `json:"actorName"`
	Label       string         `json:"label"`
	Reference   string         `json:"reference"`
	Summary     string         `json:"summary"`
	SubjectType string         `json:"subjectType"`
	SubjectID   string         `json:"subjectId"`
	Historical  bool           `json:"historical"`
}
type Repository interface {
	Append(context.Context, Entry) error
	Entry(context.Context, string) (Entry, error)
}
