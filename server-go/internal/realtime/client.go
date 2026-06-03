package realtime

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	maxMessageSize = 4096
	sendBufferSize = 256
)

type Client struct {
	hub       *Hub
	auctionID int64
	token     string
	lastSeq   int64

	conn *websocket.Conn
	send chan []byte
}

func NewClient(hub *Hub, auctionID int64, token string, lastSeq int64, conn *websocket.Conn) *Client {
	return &Client{
		hub:       hub,
		auctionID: auctionID,
		token:     token,
		lastSeq:   lastSeq,
		conn:      conn,
		send:      make(chan []byte, sendBufferSize),
	}
}

func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		if c.conn != nil {
			_ = c.conn.Close()
		}
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, payload, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))

		var message appMessage
		if err := json.Unmarshal(payload, &message); err != nil {
			slog.Debug("ignore malformed ws message", "auction_id", c.auctionID, "err", err)
			continue
		}

		switch message.Type {
		case MessageTypePing:
			c.enqueueJSON(map[string]any{
				"type":        MessageTypePong,
				"auction_id":  c.auctionID,
				"server_time": nowServerTime(),
			})
		case "ack":
			slog.Debug("ws ack received", "auction_id", c.auctionID, "seq", message.Seq)
		default:
			slog.Debug("ignore unsupported ws message", "auction_id", c.auctionID, "type", message.Type)
		}
	}
}

func (c *Client) writePump() {
	defer func() {
		if c.conn != nil {
			_ = c.conn.Close()
		}
	}()

	for payload := range c.send {
		_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
		if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			return
		}
	}
}

func (c *Client) enqueueEvent(event EventEnvelope) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	c.enqueue(payload)
}

func (c *Client) enqueueJSON(v any) {
	payload, err := json.Marshal(v)
	if err != nil {
		return
	}
	c.enqueue(payload)
}

func (c *Client) enqueue(payload []byte) {
	select {
	case c.send <- payload:
	default:
		c.hub.Unregister(c)
	}
}
