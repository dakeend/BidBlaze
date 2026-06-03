package realtime

import (
	"context"
	"sync"
)

type Hub struct {
	provider Provider

	register   chan *Client
	unregister chan *Client
	publish    chan EventEnvelope

	roomsMu sync.RWMutex
	rooms   map[int64]*Room
}

func NewHub(provider Provider) *Hub {
	if provider == nil {
		provider = StaticProvider{}
	}

	return &Hub{
		provider:   provider,
		register:   make(chan *Client),
		unregister: make(chan *Client),
		publish:    make(chan EventEnvelope, 256),
		rooms:      make(map[int64]*Room),
	}
}

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case client := <-h.register:
			room := h.room(ctx, client.auctionID)
			select {
			case room.register <- client:
			case <-ctx.Done():
				return
			}
		case client := <-h.unregister:
			if room := h.getRoom(client.auctionID); room != nil {
				select {
				case room.unregister <- client:
				case <-ctx.Done():
					return
				}
			}
		case event := <-h.publish:
			if room := h.getRoom(event.AuctionID); room != nil {
				select {
				case room.broadcast <- event:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Publish is the in-process handoff point for Role A's outbox publisher.
// A Redis pub/sub subscriber can decode the same EventEnvelope and call Publish.
func (h *Hub) Publish(event EventEnvelope) {
	h.publish <- event
}

// ForwardEvents bridges either an in-process outbox channel or a Redis pub/sub
// subscriber into the same room broadcast path.
func (h *Hub) ForwardEvents(ctx context.Context, events <-chan EventEnvelope) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			h.Publish(event)
		}
	}
}

func (h *Hub) getRoom(auctionID int64) *Room {
	h.roomsMu.RLock()
	defer h.roomsMu.RUnlock()
	return h.rooms[auctionID]
}

func (h *Hub) room(ctx context.Context, auctionID int64) *Room {
	h.roomsMu.RLock()
	room, ok := h.rooms[auctionID]
	h.roomsMu.RUnlock()
	if ok {
		return room
	}

	h.roomsMu.Lock()
	defer h.roomsMu.Unlock()
	if room, ok = h.rooms[auctionID]; ok {
		return room
	}

	room = NewRoom(auctionID)
	h.rooms[auctionID] = room
	go room.Run(ctx)
	return room
}

func (h *Hub) replayOrSnapshot(ctx context.Context, auctionID int64, lastSeq int64) []EventEnvelope {
	if lastSeq > 0 {
		replay, err := h.provider.EventsAfter(ctx, auctionID, lastSeq, defaultReplayLimit)
		if err == nil && !replay.SnapshotRequired && !replay.HasMore && len(replay.Events) > 0 {
			return replay.Events
		}
	}

	snapshot, err := h.provider.Snapshot(ctx, auctionID)
	if err != nil {
		snapshot = newSnapshotEvent(auctionID, 0)
	}
	return []EventEnvelope{snapshot}
}
