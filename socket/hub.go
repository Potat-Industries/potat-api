package socket

import (
	"net/http"
	"time"

	"potat-api/common"
	"potat-api/common/logger"
	"potat-api/common/utils"
)

type hub struct {
	clients    map[*client]bool
	broadcast  chan []byte
	register   chan *client
	unregister chan *client
}

func newHub() *hub {
	return &hub{
		broadcast:  make(chan []byte),
		register:   make(chan *client),
		unregister: make(chan *client),
		clients:    make(map[*client]bool),
	}
}

func (h *hub) run() {
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

func (h *hub) Send(message []byte) error {
	if len(h.clients) == 0 {
		return nil
	}

	h.broadcast <- message

	return nil
}

// StartServing will start the socket server on the configured port.
func StartServing(config common.Config, natsclient *utils.NatsClient) error {
	hub := newHub()
	go hub.run()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	addr := config.Socket.Host + ":" + config.Socket.Port
	logger.Info.Printf("Socket server listening on %s", addr)

	natsclient.SetProxySocketFn(hub.Send)

	server := &http.Server{
		Addr:         addr,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return server.ListenAndServe()
}
