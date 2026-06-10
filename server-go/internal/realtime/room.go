package realtime

import (
	"context"
	"encoding/json"
	"math"
	"time"
)

const (
	viewerCountInterval      = 2 * time.Second
	viewerCountDBFlushInterv = 5 * time.Second
)

// ViewerCountSink 在 viewer count 变化时回调，用于同步到数据库等外部存储。
type ViewerCountSink func(auctionID int64, count int)

type Room struct {
	auctionID int64

	register   chan *Client
	unregister chan *Client
	broadcast  chan EventEnvelope

	clients map[*Client]struct{}

	lastBusinessSeq          int64
	lastViewerBroadcastCount int
	lastViewerBroadcastAt    time.Time
	viewerDirty              bool

	onViewerChange ViewerCountSink
	lastDBFlushAt  time.Time
	lastDBFlushCnt int
}

func NewRoom(auctionID int64, sink ViewerCountSink) *Room {
	return &Room{
		auctionID:      auctionID,
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		broadcast:      make(chan EventEnvelope, 256),
		clients:        make(map[*Client]struct{}),
		viewerDirty:    true,
		onViewerChange: sink,
	}
}

func (r *Room) Run(ctx context.Context) {
	ticker := time.NewTicker(viewerCountInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			for client := range r.clients {
				close(client.send)
				delete(r.clients, client)
			}
			return
		case client := <-r.register:
			r.clients[client] = struct{}{}
			r.viewerDirty = true
			r.maybeBroadcastViewerCount(time.Now())
		case client := <-r.unregister:
			if _, ok := r.clients[client]; ok {
				delete(r.clients, client)
				close(client.send)
				r.viewerDirty = true
				r.maybeBroadcastViewerCount(time.Now())
			}
		case event := <-r.broadcast:
			r.broadcastEvent(event)
		case now := <-ticker.C:
			r.maybeBroadcastViewerCount(now)
		}
	}
}

func (r *Room) broadcastEvent(event EventEnvelope) {
	if event.Type != EventViewerCount && event.Seq > r.lastBusinessSeq {
		r.lastBusinessSeq = event.Seq
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	r.broadcastPayload(payload)
}

func (r *Room) maybeBroadcastViewerCount(now time.Time) {
	if !r.viewerDirty {
		return
	}
	if !r.lastViewerBroadcastAt.IsZero() && now.Sub(r.lastViewerBroadcastAt) < viewerCountInterval {
		return
	}

	current := len(r.clients)
	if !r.shouldBroadcastViewerCount(current) {
		r.viewerDirty = false
		return
	}

	delta := current - r.lastViewerBroadcastCount
	r.lastViewerBroadcastCount = current
	r.lastViewerBroadcastAt = now
	r.viewerDirty = false

	// 定期将真实在线人数刷入 DB，供 HTTP 轮询端（监控页）读取
	if r.onViewerChange != nil && (r.lastDBFlushAt.IsZero() || now.Sub(r.lastDBFlushAt) >= viewerCountDBFlushInterv || current != r.lastDBFlushCnt) {
		r.onViewerChange(r.auctionID, current)
		r.lastDBFlushAt = now
		r.lastDBFlushCnt = current
	}

	// viewer_count is a soft event. Seq mirrors the latest business seq and
	// must not be used by clients as the outbox compensation cursor.
	r.broadcastEvent(newViewerCountEvent(r.auctionID, r.lastBusinessSeq, current, delta))
}

func (r *Room) shouldBroadcastViewerCount(current int) bool {
	if r.lastViewerBroadcastAt.IsZero() {
		return true
	}
	if current == r.lastViewerBroadcastCount {
		return false
	}
	if r.lastViewerBroadcastCount == 0 {
		return true
	}

	change := math.Abs(float64(current-r.lastViewerBroadcastCount)) / float64(r.lastViewerBroadcastCount)
	return change >= 0.02
}

func (r *Room) broadcastPayload(payload []byte) {
	for client := range r.clients {
		select {
		case client.send <- payload:
		default:
			close(client.send)
			delete(r.clients, client)
			r.viewerDirty = true
		}
	}
}
