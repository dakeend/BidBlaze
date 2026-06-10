package outbox

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestPublisherPreservesAuctionSequence(t *testing.T) {
	store := newFakeStore(3,
		Event{ID: 2, AggregateID: 1, EventSeq: 2, Payload: []byte(`{"seq":2}`)},
		Event{ID: 1, AggregateID: 1, EventSeq: 1, Payload: []byte(`{"seq":1}`)},
		Event{ID: 3, AggregateID: 2, EventSeq: 1, Payload: []byte(`{"seq":1}`)},
	)
	sink := &recordingSink{}
	publisher := newTestPublisher(store, sink, 10)

	result, err := publisher.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if result.Published != 3 {
		t.Fatalf("published=%d, want 3", result.Published)
	}

	sink.mu.Lock()
	defer sink.mu.Unlock()
	var roomOne []int64
	for _, event := range sink.events {
		if event.AggregateID == 1 {
			roomOne = append(roomOne, event.EventSeq)
		}
	}
	if len(roomOne) != 2 || roomOne[0] != 1 || roomOne[1] != 2 {
		t.Fatalf("room order=%v, want [1 2]", roomOne)
	}
}

func TestPublisherAllowsDuplicateAfterCommitFailure(t *testing.T) {
	store := newFakeStore(3,
		Event{ID: 1, AggregateID: 1, EventSeq: 1, Payload: []byte(`{"seq":1}`)},
	)
	store.commitFailures = 1
	sink := &recordingSink{}
	publisher := newTestPublisher(store, sink, 1)

	if _, err := publisher.ProcessOnce(context.Background()); err == nil {
		t.Fatal("expected simulated commit failure")
	}
	result, err := publisher.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("second publish failed: %v", err)
	}
	if result.Published != 1 {
		t.Fatalf("published=%d, want 1", result.Published)
	}

	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.events) != 2 {
		t.Fatalf("deliveries=%d, want duplicate delivery count 2", len(sink.events))
	}
}

func TestPublisherMovesRepeatedFailureToDeadLetter(t *testing.T) {
	store := newFakeStore(2,
		Event{ID: 1, AggregateID: 1, EventSeq: 1, Payload: []byte(`{"seq":1}`)},
	)
	sink := &recordingSink{failuresRemaining: 2}
	publisher := newTestPublisher(store, sink, 1)

	first, err := publisher.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("first cycle failed: %v", err)
	}
	if first.Retried != 1 {
		t.Fatalf("retried=%d, want 1", first.Retried)
	}

	second, err := publisher.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("second cycle failed: %v", err)
	}
	if second.DeadLetter != 1 {
		t.Fatalf("dead letters=%d, want 1", second.DeadLetter)
	}
	if snapshot := publisher.Metrics(); snapshot.OutboxPendingTotal != 0 ||
		snapshot.PublishFailedTotal != 2 ||
		snapshot.DeadLetterTotal != 1 {
		t.Fatalf("unexpected metrics: %+v", snapshot)
	}
}

func TestChannelSinkDeliversEvent(t *testing.T) {
	sink := NewChannelSink(1)
	event := Event{ID: 1, AggregateID: 9, EventSeq: 3}
	if err := sink.Publish(context.Background(), event); err != nil {
		t.Fatalf("channel publish failed: %v", err)
	}
	select {
	case received := <-sink.Events():
		if received.ID != event.ID {
			t.Fatalf("received=%+v, want %+v", received, event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for channel event")
	}
}

func newTestPublisher(store Store, sink Sink, batch int) *Publisher {
	return NewPublisher(
		store,
		sink,
		&Metrics{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		time.Millisecond,
		batch,
		time.Second,
	)
}

type fakeOutboxRecord struct {
	event  Event
	status string
}

type fakeStore struct {
	mu             sync.Mutex
	records        []fakeOutboxRecord
	maxRetries     int
	commitFailures int
}

func newFakeStore(maxRetries int, events ...Event) *fakeStore {
	records := make([]fakeOutboxRecord, 0, len(events))
	for _, event := range events {
		records = append(records, fakeOutboxRecord{event: event, status: "pending"})
	}
	return &fakeStore{
		records:    records,
		maxRetries: maxRetries,
	}
}

func (s *fakeStore) ProcessNext(
	ctx context.Context,
	publish func(context.Context, Event) error,
) (ProcessResult, bool, error) {
	s.mu.Lock()
	index := s.nextEligibleIndex()
	if index < 0 {
		s.mu.Unlock()
		return ProcessResult{}, false, nil
	}
	record := s.records[index]
	s.mu.Unlock()

	startedAt := time.Now()
	publishErr := publish(ctx, record.event)
	result := ProcessResult{
		Event:          record.event,
		PublishLatency: time.Since(startedAt),
		PublishError:   publishErr,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if publishErr == nil {
		if s.commitFailures > 0 {
			s.commitFailures--
			return ProcessResult{}, true, errors.New("simulated commit failure")
		}
		s.records[index].status = "published"
		result.Outcome = OutcomePublished
		return result, true, nil
	}

	s.records[index].event.RetryCount++
	if s.records[index].event.RetryCount >= s.maxRetries {
		s.records[index].status = "failed"
		result.Outcome = OutcomeDeadLetter
	} else {
		result.Outcome = OutcomeRetry
	}
	return result, true, nil
}

func (s *fakeStore) PendingCount(context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int64
	for _, record := range s.records {
		if record.status == "pending" {
			count++
		}
	}
	return count, nil
}

func (s *fakeStore) nextEligibleIndex() int {
	best := -1
	for index, record := range s.records {
		if record.status != "pending" {
			continue
		}
		blocked := false
		for _, previous := range s.records {
			if previous.status == "pending" &&
				previous.event.AggregateID == record.event.AggregateID &&
				previous.event.EventSeq < record.event.EventSeq {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		if best < 0 || record.event.ID < s.records[best].event.ID {
			best = index
		}
	}
	return best
}

type recordingSink struct {
	mu                sync.Mutex
	events            []Event
	failuresRemaining int
}

func (s *recordingSink) Publish(_ context.Context, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	if s.failuresRemaining > 0 {
		s.failuresRemaining--
		return errors.New("simulated gateway failure")
	}
	return nil
}
