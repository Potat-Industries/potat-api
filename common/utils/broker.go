package utils

import (
	"context"
	"errors"
	"fmt"
	"potat-api/common"
	"strings"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	Conn *amqp.Connection
	proxySocketFn func(string) error
	connString string
)

func CreateBroker(
	config common.Config,
	parentContext context.Context,
) (func(), error) {
	user := config.RabbitMQ.User
	if user == "" {
		user = "guest"
	}

	password := config.RabbitMQ.Password
	if password == "" {
		password = "guest"
	}

	host := config.RabbitMQ.Host
	if host == "" {
		host = "localhost"
	}

	port := config.RabbitMQ.Port
	if port == "" {
		port = "5672"
	}

	connString = fmt.Sprintf(
		"amqp://%s:%s@%s:%s/", user, password, host, port,
	)

	ctx, cancelConsumer := context.WithCancel(parentContext)

	go func() {
		for {
			err := runBroker(ctx)
			if err != nil {
				Warn.Printf("RabbitMQ connection error: %v", err)
			}

			select {
			case <-parentContext.Done():
				return
			default: {
				Warn.Println("RabbitMQ connection lost, reconnecting...")
				cancelConsumer()

				time.Sleep(5 * time.Second)

				ctx, cancelConsumer = context.WithCancel(parentContext)
			}
			}
		}
	}()

	cleanup := func() {
		if Conn != nil {
			_ = Conn.Close()
			Warn.Printf("RabbitMQ connection closed")
		}
	}

	return cleanup, nil
}

func runBroker(ctx context.Context) error {
	var err error
	Conn, err = getConnection()
	if err != nil {
			return err
	}
	Info.Printf("Connected to RabbitMQ")

	err = PublishToQueue(
		context.Background(),
		"connected",
		5 * time.Second,
	)
	if err != nil {
		return err
	}

	ch, err := Conn.Channel()
	if err != nil {
			return err
	}

	queue, err := declare("potat-api", ch)
	if err != nil {
			return err
	}
	proxyQueue, err := declare("proxy-socket", ch)
	if err != nil {
			return err
	}
	err = ch.QueueBind(queue.Name, queue.Name, "potat-api", false, nil)
	if err != nil {
			return err
	}

	msgs, err := consume(ctx, queue, ch)
	if err != nil {
			return err
	}
	proxyMsgs, err := consume(ctx, proxyQueue, ch)
	if err != nil {
			return err
	}

	go processQueue(ctx, msgs, proxyMsgs)

	notifyClose := make(chan *amqp.Error, 1)
	ch.NotifyClose(notifyClose)

	select {
	case <-ctx.Done():
			Info.Println("Consumer context canceled")
			return nil
	case err := <-notifyClose:
			if err != nil {
					Warn.Printf("Channel closed: %v", err)
			}
			return err
	}
}

func getConnection() (*amqp.Connection, error) {
	conn, err := amqp.Dial(connString)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func SetProxySocketFn(fn func(string) error) {
	proxySocketFn = fn
}

func Stop() {
	if Conn != nil {
		_ = Conn.Close()
		Warn.Printf("RabbitMQ connection closed")
	}
}

func declare(queue string, channel *amqp.Channel) (amqp.Queue, error) {
	return channel.QueueDeclare(
		queue,
		true,
		false,
		false,
		false,
		nil,
	)
}

func consume(
	ctx context.Context,
	queue amqp.Queue,
	channel *amqp.Channel,
) (<-chan amqp.Delivery, error) {
	return channel.ConsumeWithContext(
		ctx,
		queue.Name,
		"",
		false,
		false,
		true,
		false,
		nil,
	)
}

func processQueue(ctx context.Context, msgs, proxyMsgs <-chan amqp.Delivery) {
	for {
		select {
		case <-ctx.Done():
			return;
		case msg := <-msgs:
			if msg.Body == nil {
				_ = msg.Reject(false)
				continue
			}

			if msg.CorrelationId == "potatbotat" {
				err := msg.Ack(false)
				if err != nil {
					Warn.Printf("Failed to acknowledge message: %v", err)
				} else {
					handleMessage(string(msg.Body))
				}
			} else {
				err := msg.Reject(true)
				if err != nil {
					Warn.Printf("Failed to reject and requeue message: %v", err)
				}
			}
		case msg := <-proxyMsgs:
			if msg.Body == nil {
				_ = msg.Reject(false)
				continue
			}

			if proxySocketFn != nil {
				err := msg.Ack(false)
				if err != nil {
					Warn.Printf("Failed to acknowledge message: %v", err)
				}
				err = proxySocketFn(string(msg.Body))
				if err != nil {
					Warn.Printf("Failed to send message to socket: %v", err)
				}
			} else {
				Warn.Println("Proxy socket function not set")
				err := msg.Reject(true)
				if err != nil {
					Warn.Printf("Failed to acknowledge message: %v", err)
				}
			}
		}
	}
}

func handleMessage(message string) {
	if message == "" {
		return
	}
	parts := strings.Split(message, ":")

	var topic string
	if len(parts) >= 1 {
		topic = parts[0]
		message = strings.Join(parts[1:], ":")
	} else {
		topic = message
	}

	Debug.Printf("[x] Received %s", message)

	switch topic {
	case "ping":
	  err := PublishToQueue(context.Background(), "pong", 5 * time.Second)
	  if err != nil {
			Warn.Printf("Failed to send pong: %v", err)
		}
	case "pong":
		Debug.Println("PotatBotat Reconnected to API")
		err := PublishToQueue(context.Background(), "ping", 5 * time.Second)
		if err != nil {
			Warn.Printf("Failed to send ping: %v", err)
		}
	default:
		Debug.Printf("[x] Unrecognized topic: %s", topic)
	}
}

func PublishToQueue(
	ctx context.Context,
	message string,
	ttl time.Duration,
) error {
	if Conn == nil {
		Warn.Println("RabbitMQ connection not established")
		return nil
	}

	ch, err := Conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	ctx, cancel := context.WithTimeout(ctx, ttl + time.Second)
	defer cancel()

	err = ch.PublishWithContext(ctx,
		"potat-api",
		"potat-api",
		false,
		false,
		amqp.Publishing{
			ContentType: 	 	"text/plain",
			Body:        	 	[]byte(message),
			CorrelationId: 	"potat-api",
			Expiration:  	 	fmt.Sprintf("%d", ttl.Milliseconds()),
		},
	)
	if err != nil {
		return err
	}

	Debug.Printf("[x] Sent %s", message)
	return nil
}

func RequestManager(
	ctx context.Context,
	ttl time.Duration,
	request string,
	callback func([]byte),
) error {
	if request == "" {
		return errors.New("empty request")
	}

	ch, err := Conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open RabbitMQ channel: %w", err)
	}

	defer func() {
		err := ch.Close()
		if err != nil {
			Warn.Printf("Failed to close channel: %v", err)
		}
	}()

	queue, err := declare("job-requests", ch)
	if err != nil {
		return fmt.Errorf("failed to declare reply queue: %w", err)
	}

	msgs, err := consume(ctx, queue, ch)
	if err != nil {
		return fmt.Errorf("failed to consume from reply queue: %w", err)
	}

	correlationID := uuid.New().String()

	err = ch.PublishWithContext(
		ctx,
		"job-requests",
		"job-requests",
		false,
		false,
		amqp.Publishing{
			ContentType:   "text/plain",
			CorrelationId: correlationID,
			ReplyTo:       "job-requests",
			Body:          []byte(request),
			Expiration:    fmt.Sprintf("%d", ttl.Milliseconds()),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish request: %w", err)
	}

	timeout := time.NewTimer(ttl)
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return fmt.Errorf("request timed out after %s", ttl)
		case msg := <-msgs:
			if msg.CorrelationId == correlationID {
				callback(msg.Body)
				timeout.Stop()
				return nil
			}
		}
	}
}
