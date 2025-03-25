// Package socket provides a websocket server for sending and receiving messages, as a proxy from NATS.
package socket

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"potat-api/common/logger"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = time.Minute
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
	socketTTL      = time.Hour
)

type eventCodes uint16

const (
	hello         eventCodes = 4444
	receivedData  eventCodes = 4000
	reconnect     eventCodes = 4001
	unknownError  eventCodes = 4002
	invalidOrigin eventCodes = 4003
	dispatch      eventCodes = 4004
	heartbeat     eventCodes = 4005
	malformedData eventCodes = 4006
	unauthorized  eventCodes = 4007
)

type potatMessage struct {
	Data   any        `json:"data"`
	Topic  string     `json:"topic"`
	Opcode eventCodes `json:"opcode"`
}

type client struct {
	hub         *hub
	conn        *websocket.Conn
	send        chan []byte
	closeSignal chan struct{}
	id          string
	closeOnce   sync.Once
	writeMutex  sync.Mutex
}

func (c *client) sendJSON(data *potatMessage) error {
	message, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return c.sendMessage(websocket.TextMessage, message)
}

func (c *client) sendMessage(messageType int, data []byte) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	if err := c.conn.WriteMessage(messageType, data); err != nil {
		return err
	}

	return nil
}

//nolint:unused // todo: remove?
func (c *client) sendEvent(opcode eventCodes, topic string, data any) error {
	response := &potatMessage{
		Opcode: opcode,
		Topic:  topic,
		Data:   data,
	}

	return c.sendJSON(response)
}

func (c *client) pongHandler(string) error {
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		logger.Warn.Println("Failed setting read deadline", err)

		return err
	}

	return nil
}

func (c *client) writePingPumperDumper9000() {
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
				logger.Warn.Println("Failed to send ping message:", err)
			}
		case <-ttlTicker.C:
			logger.Warn.Printf("Client %s has reached TTL, sending reconnect message...", c.id)

			response := &potatMessage{
				Opcode: reconnect,
				Topic:  "Please reconnect right NEOW!",
			}

			time.Sleep(2 * time.Second)
			if err := c.sendJSON(response); err != nil {
				logger.Warn.Printf("Failed to send reconnect message: %v", err)
			}

			c.closeHandler("TTL reached")

			return
		case message, ok := <-c.send:
			if !ok {
				return
			}

			if err := c.sendMessage(websocket.TextMessage, message); err != nil {
				logger.Warn.Printf("Failed writing message: %v", err)

				return
			}
		}
	}
}

func (c *client) closeHandler(reason string) {
	if reason == "" {
		reason = "No reason provided"
	}

	c.closeOnce.Do(func() {
		close(c.closeSignal)
		c.hub.unregister <- c

		if err := c.conn.Close(); err != nil {
			logger.Warn.Println("Failed closing connection: ", err)
		} else {
			logger.Warn.Printf("Socket closed for %s, reason: %s", c.id, reason)
		}
	})
}

func (c *client) readPump() {
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
					logger.Warn.Printf("Normal close error: %v", err)
				} else if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					logger.Warn.Printf("Unexpected close error: %v", err)
				}
				c.closeHandler("Socket already closed")

				break
			}

			logger.Warn.Printf("Received message from %s, closing connection...", c.id)
			response := &potatMessage{
				Opcode: receivedData,
				Topic:  "Potat socket does not accept incoming messages 😡",
			}

			if err := c.sendJSON(response); err != nil {
				logger.Warn.Printf("Failed to send incoming messages response: %v", err)
			}

			c.closeHandler("Incoming messages not allowed")

			return
		}
	}
}

func serveWs(hub *hub, writer http.ResponseWriter, request *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(_ *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(writer, request, nil)
	if err != nil {
		logger.Warn.Println("Failed to upgrade connection:", err)

		return
	}

	actor := request.RemoteAddr
	if request.Header.Get("cf-connecting-ip") != "" {
		actor = request.Header.Get("cf-connecting-ip")
	}

	client := &client{
		hub:         hub,
		conn:        conn,
		send:        make(chan []byte, 1024),
		id:          actor,
		closeSignal: make(chan struct{}),
	}
	client.hub.register <- client

	go client.readPump()
	go client.writePingPumperDumper9000()

	err = client.sendJSON(&potatMessage{
		Opcode: hello,
		Topic:  "Welcome to Potat socket!",
	})
	if err != nil {
		logger.Warn.Println("Failed to send welcome message:", err)
	}

	logger.Info.Printf("Potat socket connection from %s", actor)
}
