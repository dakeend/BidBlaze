package worker

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestConcurrentWorkersDoNotDuplicateOrder(t *testing.T) {
	store := newFakeStore(fakeAuction{
		id:        1,
		status:    "active",
		dueToEnd:  true,
		hasLeader: true,
	})
	store.listBarrier = make(chan struct{})

	runnerA := NewRunner(store, discardLogger(), 10*time.Millisecond, 100)
	runnerB := NewRunner(store, discardLogger(), 10*time.Millisecond, 100)

	var wg sync.WaitGroup
	wg.Add(2)
	results := make(chan CycleResult, 2)
	errors := make(chan error, 2)
	for _, runner := range []*Runner{runnerA, runnerB} {
		go func(current *Runner) {
			defer wg.Done()
			result, err := current.ProcessOnce(context.Background())
			results <- result
			errors <- err
		}(runner)
	}
	wg.Wait()
	close(results)
	close(errors)

	for err := range errors {
		if err != nil {
			t.Fatalf("worker failed: %v", err)
		}
	}

	totalEnded := 0
	totalOrders := 0
	for result := range results {
		totalEnded += result.Ended
		totalOrders += result.Orders
	}
	if totalEnded != 1 {
		t.Fatalf("ended transitions=%d, want 1", totalEnded)
	}
	if totalOrders != 1 {
		t.Fatalf("orders reported=%d, want 1", totalOrders)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if store.auction.status != "ended" {
		t.Fatalf("status=%s, want ended", store.auction.status)
	}
	if store.orderCount != 1 {
		t.Fatalf("order count=%d, want 1", store.orderCount)
	}
	if store.endedEventCount != 1 {
		t.Fatalf("AuctionEnded events=%d, want 1", store.endedEventCount)
	}
}

func TestStartDueCreatesOneStartedEvent(t *testing.T) {
	store := newFakeStore(fakeAuction{
		id:         1,
		status:     "pending",
		dueToStart: true,
	})
	runner := NewRunner(store, discardLogger(), 10*time.Millisecond, 100)

	result, err := runner.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("worker failed: %v", err)
	}
	if result.Started != 1 {
		t.Fatalf("started=%d, want 1", result.Started)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if store.auction.status != "active" {
		t.Fatalf("status=%s, want active", store.auction.status)
	}
	if store.startedEventCount != 1 {
		t.Fatalf("AuctionStarted events=%d, want 1", store.startedEventCount)
	}
}

func TestEndWithoutLeaderCreatesNoOrder(t *testing.T) {
	store := newFakeStore(fakeAuction{
		id:       1,
		status:   "active",
		dueToEnd: true,
	})
	runner := NewRunner(store, discardLogger(), 10*time.Millisecond, 100)

	result, err := runner.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("worker failed: %v", err)
	}
	if result.Ended != 1 || result.Orders != 0 {
		t.Fatalf("unexpected cycle result: %+v", result)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if store.orderCount != 0 {
		t.Fatalf("order count=%d, want 0", store.orderCount)
	}
	if store.endedEventCount != 1 {
		t.Fatalf("AuctionEnded events=%d, want 1", store.endedEventCount)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeAuction struct {
	id         int64
	status     string
	dueToStart bool
	dueToEnd   bool
	hasLeader  bool
}

type fakeStore struct {
	mu                sync.Mutex
	auction           fakeAuction
	orderCount        int
	startedEventCount int
	endedEventCount   int
	nextOrderID       int64
	listBarrier       chan struct{}
	listCalls         int
}

func newFakeStore(auction fakeAuction) *fakeStore {
	return &fakeStore{
		auction:     auction,
		nextOrderID: 1,
	}
}

func (s *fakeStore) StartDue(context.Context, int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.auction.status != "pending" || !s.auction.dueToStart {
		return 0, nil
	}
	s.auction.status = "active"
	s.startedEventCount++
	return 1, nil
}

func (s *fakeStore) ListDueEndIDs(context.Context, int) ([]int64, error) {
	s.mu.Lock()
	ids := []int64{}
	if s.auction.status == "active" && s.auction.dueToEnd {
		ids = append(ids, s.auction.id)
	}
	barrier := s.listBarrier
	if barrier != nil {
		s.listCalls++
		if s.listCalls == 2 {
			close(barrier)
		}
	}
	s.mu.Unlock()

	if barrier != nil {
		<-barrier
	}
	return ids, nil
}

func (s *fakeStore) EndOne(context.Context, int64) (EndResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.auction.status != "active" || !s.auction.dueToEnd {
		return EndResult{}, nil
	}

	s.auction.status = "ended"
	s.endedEventCount++
	if !s.auction.hasLeader {
		return EndResult{Ended: true}, nil
	}

	orderID := s.nextOrderID
	s.nextOrderID++
	s.orderCount++
	return EndResult{Ended: true, OrderID: &orderID}, nil
}
