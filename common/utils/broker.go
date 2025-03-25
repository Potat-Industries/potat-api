// Package utils provides utility functions and types for all routes.
package utils

import (
	"context"
	"errors"
	"fmt"
	"time"

	nats "github.com/nats-io/nats.go"
	"potat-api/common/logger"
)

var errNatsNotConnected = errors.New("NATS client not connected")

// NatsClient is a wrapper around the NATS client to handle message publishing and subscription.
type NatsClient struct {
	client        *nats.Conn
	proxySocketFn func([]byte) error
}

// CreateNatsBroker initializes a NATS client and starts a goroutine to handle reconnections.
func CreateNatsBroker(
	parentContext context.Context,
) (*NatsClient, error) {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		return &NatsClient{}, err
	}

	client := NatsClient{
		client: nc,
	}

	ctx, cancel := context.WithCancel(parentContext)

	go func() {
		for {
			err := client.subNatsStream(ctx)
			if err != nil {
				logger.Warn.Printf("NATS connection error: %v", err)
			}

			select {
			case <-parentContext.Done():
				return
			default:
				{
					logger.Warn.Println("NATS connection lost, reconnecting...")
					cancel()

					time.Sleep(5 * time.Second)

					ctx, cancel = context.WithCancel(parentContext)
				}
			}
		}
	}()

	return &client, nil
}

func (n *NatsClient) subNatsStream(ctx context.Context) error {
	sub, err := n.client.Subscribe("potatbotat.>", n.handleMessage)
	if err != nil {
		return err
	}
	defer func() {
		err = sub.Unsubscribe()
		if err != nil {
			logger.Warn.Printf("Failed to unsubscribe NATs topic: %v", err)
		}
	}()

	err = n.client.Publish("potat-api.connected", []byte(nil))
	if err != nil {
		logger.Warn.Printf("Failed to publish connected message: %v", err)
	}

	<-ctx.Done()

	return nil
}

// SetProxySocketFn sets the function to handle proxy socket messages.
func (n *NatsClient) SetProxySocketFn(fn func([]byte) error) {
	n.proxySocketFn = fn
}

// Publish sends a message to the specified topic on the NATS server.
func (n *NatsClient) Publish(topic string, data []byte) error {
	if n.client == nil {
		return errNatsNotConnected
	}

	err := n.client.Publish(topic, data)
	if err != nil {
		return err
	}
	logger.Debug.Printf("[x] Sent %s", data)

	return nil
}

func (n *NatsClient) onPing() {
	err := n.Publish("potat-api.pong", []byte(nil))
	if err != nil {
		logger.Warn.Printf("Failed to send pong: %v", err)
	}
}

func (n *NatsClient) onPong() {
	logger.Debug.Println("PotatBotat Reconnected to API")
	err := n.Publish("potat-api.ping", []byte(nil))
	if err != nil {
		logger.Warn.Printf("Failed to send ping: %v", err)
	}
}

func (n *NatsClient) onProxySocket(message *nats.Msg) {
	if n.proxySocketFn != nil {
		err := n.proxySocketFn(message.Data)
		if err != nil {
			logger.Warn.Printf("Failed to proxy socket: %v", err)
		}
	}
}

func (n *NatsClient) handleMessage(message *nats.Msg) {
	if message == nil {
		return
	}

	switch message.Subject {
	case "potatbotat.ping":
		n.onPing()
	case "potatbotat.pong":
		n.onPong()
	case "potatbotat.proxy-socket":
		n.onProxySocket(message)
	case "potatbotat.api-request":
	default:
		logger.Debug.Printf("[x] Unrecognized topic: %s", message.Subject)
	}
}

// BridgeRequest sends a request to the NATS server and waits for a response.
func BridgeRequest(
	ttl time.Duration,
	request string,
) ([]byte, error) {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		return nil, fmt.Errorf("failed to publish request: %w", err)
	}

	response, err := nc.Request(
		"potat-api.job-request",
		[]byte(request),
		ttl,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to publish request: %w", err)
	}

	return response.Data, nil
}
