package outbox

import (
	"encoding/json"
	"time"
)

type Event struct {
	ID            int64
	AggregateType string
	AggregateID   int64
	EventType     string
	EventSeq      int64
	Payload       json.RawMessage
	RetryCount    int
	CreatedAt     time.Time
}

type Outcome string

const (
	OutcomePublished  Outcome = "published"
	OutcomeRetry      Outcome = "retry"
	OutcomeDeadLetter Outcome = "dead_letter"
)

type ProcessResult struct {
	Event          Event
	Outcome        Outcome
	PublishLatency time.Duration
	PublishError   error
}

type BatchResult struct {
	Published  int
	Retried    int
	DeadLetter int
}
