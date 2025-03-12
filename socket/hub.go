package socket

import (
	"errors"
	"net/http"

	"potat-api/common"
	"potat-api/common/utils"
)

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

var hub *Hub

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

func Send(message []byte) error {
	if hub == nil {
		return errors.New("hub is not initialized!")
	}

	if len(hub.clients) == 0 {
		return nil
	}

	hub.broadcast <- message

	return nil
}

func StartServing(config common.Config, natsClient *utils.NatsClient) error {
	hub = newHub()
	go hub.run()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	addr := config.Socket.Host + ":" + config.Socket.Port
	utils.Info.Printf("Socket server listening on %s", addr)

	natsClient.SetProxySocketFn(Send)

	return http.ListenAndServe(addr, nil)
}
