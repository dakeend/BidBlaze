package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestCheckOriginUsesConfiguredOrigins(t *testing.T) {
	t.Setenv("WS_ALLOWED_ORIGINS", "https://auction.example.com, http://localhost:5173")

	allowed := httptest.NewRequest("GET", "/ws/auction/1", nil)
	allowed.Header.Set("Origin", "https://auction.example.com")
	if !checkOrigin(allowed) {
		t.Fatal("expected configured origin to be allowed")
	}

	denied := httptest.NewRequest("GET", "/ws/auction/1", nil)
	denied.Header.Set("Origin", "https://evil.example.com")
	if checkOrigin(denied) {
		t.Fatal("expected unconfigured origin to be denied")
	}
}

func TestCheckOriginFallsBackToCORSOrigins(t *testing.T) {
	t.Setenv("WS_ALLOWED_ORIGINS", "")
	t.Setenv("CORS_ORIGINS", "https://frontend.example.com")

	request := httptest.NewRequest("GET", "/ws/auction/1", nil)
	request.Header.Set("Origin", "https://frontend.example.com")
	if !checkOrigin(request) {
		t.Fatal("expected CORS_ORIGINS origin to be allowed")
	}
}

func TestHubRoomCreationIsConcurrentSafe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := NewHub(nil, nil)
	const workers = 32
	rooms := make(chan *Room, workers)

	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			rooms <- hub.room(ctx, 42)
		}()
	}
	wg.Wait()
	close(rooms)

	var first *Room
	for room := range rooms {
		if first == nil {
			first = room
			continue
		}
		if first != room {
			t.Fatal("expected concurrent room calls to return the same room instance")
		}
	}
}

func TestHubPublishBroadcastsOnlyMatchingRoom(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := NewHub(nil, nil)
	go hub.Run(ctx)

	roomOne := &Client{hub: hub, auctionID: 1, send: make(chan []byte, 8)}
	roomTwo := &Client{hub: hub, auctionID: 2, send: make(chan []byte, 8)}
	hub.Register(roomOne)
	hub.Register(roomTwo)

	event := EventEnvelope{
		Type:       "bid_update",
		EventID:    "evt_1_1",
		AuctionID:  1,
		Seq:        1,
		ServerTime: nowServerTime(),
		Data:       json.RawMessage(`{"current_price":100}`),
	}
	hub.Publish(event)

	got := waitForEvent(t, roomOne.send, "bid_update")
	if got.AuctionID != 1 || got.Seq != 1 {
		t.Fatalf("unexpected event: %+v", got)
	}

	assertNoEventType(t, roomTwo.send, "bid_update")
}

func TestHubSendsViewerCountSoftEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := NewHub(nil, nil)
	go hub.Run(ctx)

	client := &Client{hub: hub, auctionID: 9, send: make(chan []byte, 8)}
	hub.Register(client)

	got := waitForEvent(t, client.send, EventViewerCount)
	if got.AuctionID != 9 {
		t.Fatalf("unexpected auction id: %d", got.AuctionID)
	}
	if got.Seq != 0 {
		t.Fatalf("viewer_count should not advance outbox seq, got %d", got.Seq)
	}
}

func TestRoomStopsClientsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	room := NewRoom(7)
	go room.Run(ctx)

	client := &Client{auctionID: 7, send: make(chan []byte, 8)}
	room.register <- client
	waitForEvent(t, client.send, EventViewerCount)

	cancel()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-client.send:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for room to close client channel")
		}
	}
}

func TestReplayFallsBackToSnapshot(t *testing.T) {
	hub := NewHub(nil, nil)
	events := hub.replayOrSnapshot(context.Background(), 3, 99)
	if len(events) != 1 {
		t.Fatalf("expected one snapshot event, got %d", len(events))
	}
	if events[0].Type != EventSnapshot {
		t.Fatalf("expected snapshot, got %s", events[0].Type)
	}
}

func TestReplayFallsBackToSnapshotWhenReplayHasMore(t *testing.T) {
	hub := NewHub(&fakeProvider{, nil
		result: ReplayResult{
			Events: []EventEnvelope{{
				Type:       "bid_update",
				EventID:    "evt_100",
				AuctionID:  3,
				Seq:        100,
				ServerTime: nowServerTime(),
				Data:       json.RawMessage(`{}`),
			}},
			HasMore: true,
		},
	})

	events := hub.replayOrSnapshot(context.Background(), 3, 99)
	if len(events) != 1 {
		t.Fatalf("expected one snapshot event, got %d", len(events))
	}
	if events[0].Type != EventSnapshot {
		t.Fatalf("expected snapshot, got %s", events[0].Type)
	}
}

func TestServeAuctionSendsSnapshotThenPong(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := NewHub(nil, nil)
	go hub.Run(ctx)

	router := gin.New()
	RegisterRoutes(router, hub)

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/auction/5?token=mock-token-user-001"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	var first EventEnvelope
	readJSONMessage(t, conn, &first)
	if first.Type != EventSnapshot {
		t.Fatalf("first message should be snapshot, got %s", first.Type)
	}

	if err := conn.WriteJSON(map[string]any{"type": MessageTypePing}); err != nil {
		t.Fatalf("write ping: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for pong")
		default:
			var msg map[string]any
			readJSONMessage(t, conn, &msg)
			if msg["type"] == MessageTypePong {
				return
			}
		}
	}
}

func TestServeEventsReturnsReplayResult(t *testing.T) {
	gin.SetMode(gin.TestMode)

	provider := &fakeProvider{
		result: ReplayResult{
			Events: []EventEnvelope{{
				Type:       "bid_update",
				EventID:    "evt_11",
				AuctionID:  1,
				Seq:        11,
				ServerTime: nowServerTime(),
				Data:       json.RawMessage(`{"current_price":100}`),
			}},
			HasMore: true,
		},
	}
	hub := NewHub(provider, nil)

	router := gin.New()
	RegisterRoutes(router, hub)

	request := httptest.NewRequest(http.MethodGet, "/api/auctions/1/events?after_seq=10&limit=999", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if provider.auctionID != 1 || provider.afterSeq != 10 || provider.limit != maxReplayLimit {
		t.Fatalf("unexpected provider args: auction=%d after=%d limit=%d", provider.auctionID, provider.afterSeq, provider.limit)
	}

	var body struct {
		Code int `json:"code"`
		Data struct {
			Events           []EventEnvelope `json:"events"`
			HasMore          bool            `json:"has_more"`
			SnapshotRequired bool            `json:"snapshot_required"`
			ServerTime       string          `json:"server_time"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != 0 || !body.Data.HasMore || body.Data.SnapshotRequired {
		t.Fatalf("unexpected replay flags: %+v", body.Data)
	}
	if len(body.Data.Events) != 1 || body.Data.Events[0].Seq != 11 {
		t.Fatalf("unexpected events: %+v", body.Data.Events)
	}
	if body.Data.ServerTime == "" {
		t.Fatal("expected server_time")
	}
}

func TestServeEventsRequiresAfterSeq(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterRoutes(router, NewHub(nil, nil))

	request := httptest.NewRequest(http.MethodGet, "/api/auctions/1/events", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestServeEventsStaticProviderRequestsSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterRoutes(router, NewHub(nil, nil))

	request := httptest.NewRequest(http.MethodGet, "/api/auctions/1/events?after_seq=10", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var body struct {
		Data struct {
			SnapshotRequired bool `json:"snapshot_required"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Data.SnapshotRequired {
		t.Fatal("expected static provider to require snapshot")
	}
}

func waitForEvent(t *testing.T, ch <-chan []byte, eventType string) EventEnvelope {
	t.Helper()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case payload := <-ch:
			var event EventEnvelope
			if err := json.Unmarshal(payload, &event); err != nil {
				continue
			}
			if event.Type == eventType {
				return event
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", eventType)
		}
	}
}

type fakeProvider struct {
	result ReplayResult

	auctionID int64
	afterSeq  int64
	limit     int
}

func (p *fakeProvider) Snapshot(_ context.Context, auctionID int64) (EventEnvelope, error) {
	return newSnapshotEvent(auctionID, 0), nil
}

func (p *fakeProvider) EventsAfter(_ context.Context, auctionID int64, afterSeq int64, limit int) (ReplayResult, error) {
	p.auctionID = auctionID
	p.afterSeq = afterSeq
	p.limit = limit
	return p.result, nil
}

func readJSONMessage(t *testing.T, conn *websocket.Conn, v any) {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	if err := conn.ReadJSON(v); err != nil {
		t.Fatalf("read websocket json: %v", err)
	}
}

func assertNoEventType(t *testing.T, ch <-chan []byte, eventType string) {
	t.Helper()

	timeout := time.After(150 * time.Millisecond)
	for {
		select {
		case payload := <-ch:
			var event EventEnvelope
			if err := json.Unmarshal(payload, &event); err != nil {
				continue
			}
			if event.Type == eventType {
				t.Fatalf("unexpected %s event: %+v", eventType, event)
			}
		case <-timeout:
			return
		}
	}
}
