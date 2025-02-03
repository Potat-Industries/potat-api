package socket

import (
	"sync"
	"time"
	"net/http"
	"encoding/json"

	"potat-api/common/utils"

	"github.com/gorilla/websocket"
)

const (
	writeWait = 10 * time.Second
	pongWait = time.Minute
	pingPeriod = (pongWait * 9) / 10
	maxMessageSize = 512
	socketTTL = time.Hour
)

type EventCodes uint16

const (
	HELLO 				 EventCodes = 4444
	RECIEVED_DATA  EventCodes = 4000
	RECONNECT      EventCodes = 4001
	UNKNOWN_ERROR  EventCodes = 4002
	INVALID_ORIGIN EventCodes = 4003
	DISPATCH       EventCodes = 4004
	HEARTBEAT      EventCodes = 4005
	MALFORMED_DATA EventCodes = 4006
	UNAUTHORIZED   EventCodes = 4007
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type PotatMessage struct {
	Opcode EventCodes `json:"opcode"`
	Topic  string     `json:"topic"`
	Data	 any        `json:"data"`
}

type Client struct {
	hub *Hub
	conn *websocket.Conn
	send chan []byte
	id string
	writeMutex sync.Mutex
	closeOnce   sync.Once
	closeSignal chan struct{}
}

func (c *Client) sendJSON(data *PotatMessage) error {
	message, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return c.sendMessage(websocket.TextMessage, message)
}

func (c *Client) sendMessage(messageType int, data []byte) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	if err := c.conn.WriteMessage(messageType, data); err != nil {
		return err
	}


	return nil
}

//nolint:unused // todo: remove?
func (c *Client) sendEvent(opcode EventCodes, topic string, data any) error {
	response := &PotatMessage{
		Opcode: opcode,
		Topic:  topic,
		Data:   data,
	}

	return c.sendJSON(response)
}

func (c *Client) pongHandler(string) error {
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		utils.Warn.Println("Failed setting read deadline", err)
		return err
	}
	return nil
}

func (c *Client) writePingPumperDumper9000() {
	pingTicker := time.NewTicker(pingPeriod)
	ttlTicker := time.NewTicker(socketTTL)

	defer func() {
		ttlTicker.Stop()
	  pingTicker.Stop()
	}()

	for {
		select {
		case <-c.closeSignal:
			return
		case <-pingTicker.C:
			if err := c.sendMessage(websocket.PingMessage, nil); err != nil {
				utils.Warn.Println("Failed to send ping message:", err)
			}
		case <-ttlTicker.C:
			utils.Warn.Printf("Client %s has reached TTL, sending reconnect message...", c.id)

			response := &PotatMessage{
				Opcode: RECONNECT,
				Topic:  "Please reconnect right NEOW!",
			}

			time.Sleep(2 * time.Second)
			if err := c.sendJSON(response); err != nil {
				utils.Warn.Printf("Failed to send reconnect message: %v", err)
			}

			c.closeHandler("TTL reached")
			return
		case message, ok := <-c.send:
			if !ok {
				return
			}

			if err := c.sendMessage(websocket.TextMessage, message); err != nil {
				utils.Warn.Printf("Failed writing message: %v", err)
				return
			}
		}
	}
}

func (c *Client) closeHandler(reason string) {
	if reason == "" {
		reason = "No reason provided"
	}

	c.closeOnce.Do(func() {
		close(c.closeSignal)
		c.hub.unregister <- c

		if err := c.conn.Close(); err != nil {
			utils.Warn.Println("Failed closing connection: ", err)
		} else {
			utils.Warn.Printf("Socket closed for %s, reason: %s", c.id, reason)
		}
	})
}

func (c *Client) readPump() {
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetPongHandler(c.pongHandler)

	for {
		select {
		case <-c.closeSignal:
			return
		default:
			_, _, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					utils.Warn.Printf("Normal close error: %v", err)
				} else if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					utils.Warn.Printf("Unexpected close error: %v", err)
				}
				c.closeHandler("Socket already closed")
				break
			}

			utils.Warn.Printf("Received message from %s, closing connection...", c.id)
			response := &PotatMessage{
				Opcode: RECIEVED_DATA,
				Topic:  "Potat socket does not accept incoming messages 😡",
			}

			if err := c.sendJSON(response); err != nil {
				utils.Warn.Printf("Failed to send incoming messages response: %v", err)
			}

			c.closeHandler("Incoming messages not allowed")
			return
		}
	}
}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		utils.Warn.Println("Failed to upgrade connection:", err)
		return
	}

	actor := r.RemoteAddr
	if r.Header.Get("cf-connecting-ip") != "" {
		actor = r.Header.Get("cf-connecting-ip")
	}

	client := &Client{
		hub:         hub,
		conn:        conn,
		send:        make(chan []byte, 1024),
		id:          actor,
		closeSignal: make(chan struct{}),
	}
	client.hub.register <- client

	go client.readPump()
	go client.writePingPumperDumper9000()

	err = client.sendJSON(&PotatMessage{
		Opcode: HELLO,
		Topic:  "Welcome to Potat socket!",
	})
	if err != nil {
		utils.Warn.Println("Failed to send welcome message:", err)
	}

	utils.Info.Printf("Potat socket connection from %s", actor)
}

