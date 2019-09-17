package ws

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	maxMessageSize = 256 * 256
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  maxMessageSize,
	WriteBufferSize: maxMessageSize,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// requestHandler represents a handler for the ws request.
type requestHandler func(*Connection, []byte)

// Connection represents a single clients tile dispatcher.
type Connection struct {
	conn    *websocket.Conn
	mu      *sync.Mutex
	handler requestHandler
}

// NewConnection returns a pointer to a new tile dispatcher object.
func NewConnection(w http.ResponseWriter, r *http.Request, handler requestHandler) (*Connection, error) {
	protocol := r.Header.Get("Sec-WebSocket-Protocol")
	var responseHeader http.Header
	if protocol != "" {
		responseHeader = http.Header{
			"Sec-WebSocket-Protocol": []string{protocol},
		}
	}
	// open a websocket connection
	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		return nil, err
	}
	// set the message read limit
	conn.SetReadLimit(maxMessageSize)
	return &Connection{
		conn:    conn,
		handler: handler,
		mu:      &sync.Mutex{},
	}, nil
}

// ListenAndRespond waits on both tile request and responses and handles each
// until the websocket connection dies.
func (c *Connection) ListenAndRespond() error {
	for {
		// wait on read
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return err
		}
		// handle the message
		go c.handler(c, msg)
	}
}

// SendResponse will send a json response in a thread safe manner.
func (c *Connection) SendResponse(res interface{}) error {
	// writes are not thread safe
	c.mu.Lock()
	defer c.mu.Unlock()
	// write response to websocket
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return c.conn.WriteJSON(res)
}

// Close closes the dispatchers websocket connection.
func (c *Connection) Close() {
	// ensure we aren't closing during a write
	c.mu.Lock()
	defer c.mu.Unlock()
	// close websocket connection
	c.conn.Close()
}
