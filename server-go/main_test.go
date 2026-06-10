package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"auction-system/server-go/internal/outbox"
	"auction-system/server-go/internal/realtime"
)

func TestForwardOutboxEventsIgnoresMalformedPayload(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan outbox.Event, 1)
	hub := realtime.NewHub(realtime.StaticProvider{}, nil)

	events <- outbox.Event{
		ID:          1,
		AggregateID: 10,
		Payload:     json.RawMessage(`{`),
	}
	close(events)

	done := make(chan struct{})
	go func() {
		defer close(done)
		forwardOutboxEvents(ctx, slog.Default(), events, hub)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("forwardOutboxEvents did not return after input channel closed")
	}
}
