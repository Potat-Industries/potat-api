package socket

import (
	"net/http"
	"time"

	"github.com/Potat-Industries/potat-api/common"
	"github.com/Potat-Industries/potat-api/common/logger"
	"github.com/Potat-Industries/potat-api/common/utils"
)

type hub struct {
	clients    map[*client]bool
	broadcast  chan []byte
	register   chan *client
	unregister chan *client
	metrics    *utils.Metrics
}

func (h *hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			h.metrics.GaugeSocketConnections(float64(len(h.clients)))
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.metrics.GaugeSocketConnections(float64(len(h.clients)))
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
					h.metrics.GaugeSocketConnections(float64(len(h.clients)))
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
func StartServing(config common.Config, natsclient *utils.NatsClient, metrics *utils.Metrics) error {
	hub := &hub{
		broadcast:  make(chan []byte),
		register:   make(chan *client),
		unregister: make(chan *client),
		clients:    make(map[*client]bool),
		metrics:    metrics,
	}
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
