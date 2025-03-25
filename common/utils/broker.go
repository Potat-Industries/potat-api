package utils

import (
	"context"
	"errors"
	"fmt"
	"time"

	nats "github.com/nats-io/nats.go"
	"potat-api/common/logger"
)

type NatsClient struct {
	client        *nats.Conn
	proxySocketFn func([]byte) error
}

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
	defer sub.Unsubscribe()

	n.client.Publish("potat-api.connected", []byte(nil))

	<-ctx.Done()

	return nil
}

func (n *NatsClient) SetProxySocketFn(fn func([]byte) error) {
	n.proxySocketFn = fn
}

func (n *NatsClient) Stop() {
	if n.client != nil {
		if err := n.client.Drain(); err != nil {
			logger.Error.Printf("Failed to drain NATS connection: %v", err)
		}
		logger.Warn.Println("NATS connection closed")
	}
}

func (n *NatsClient) Publish(topic string, data []byte) error {
	if n.client == nil {
		return errors.New("NATS connection not established")
	}

	err := n.client.Publish(topic, data)
	if err != nil {
		return err
	}
	logger.Debug.Printf("[x] Sent %s", data)

	return nil
}

func (n *NatsClient) handleMessage(message *nats.Msg) {
	if message == nil {
		return
	}

	switch message.Subject {
	case "potatbotat.ping":
		err := n.Publish("potat-api.pong", []byte(nil))
		if err != nil {
			logger.Warn.Printf("Failed to send pong: %v", err)
		}
	case "potatbotat.pong":
		logger.Debug.Println("PotatBotat Reconnected to API")
		err := n.Publish("potat-api.ping", []byte(nil))
		if err != nil {
			logger.Warn.Printf("Failed to send ping: %v", err)
		}
	case "potatbotat.proxy-socket":
		if n.proxySocketFn != nil {
			err := n.proxySocketFn(message.Data)
			if err != nil {
				logger.Warn.Printf("Failed to proxy socket: %v", err)
			}
		}

		break
	case "potatbotat.api-request":
	default:
		logger.Debug.Printf("[x] Unrecognized topic: %s", message.Subject)
	}
}

func BridgeRequest(
	ctx context.Context,
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
