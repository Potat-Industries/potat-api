package utils

import (
	"context"
	"fmt"
	"potat-api/common"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	Conn *amqp.Connection
	proxySocketFn func(string) error
)

func CreateBroker(config common.Config, ctx context.Context) (func(), error) {
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

	connString := fmt.Sprintf(
		"amqp://%s:%s@%s:%s/", user, password, host, port,
	)

	var err error
	Conn, err = amqp.Dial(connString)
	if err != nil {
		return nil, err
	}

	Info.Printf("Connected to RabbitMQ")

	err = PublishToQueue(
		context.Background(),
		"connected",
		5 * time.Second,
	)
	if err != nil {
		return nil, err
	}

	err = consumeFromQueue(ctx)
	if err != nil {
		return nil, err
	}

	cleanup := func() {
		if Conn != nil {
			_ = Conn.Close()
			Warn.Printf("RabbitMQ connection closed")
		}
	}

	return cleanup, nil
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
		false,
		false,
		nil,
	)
}

func consumeFromQueue(
	ctx context.Context,
) error {
	if Conn == nil {
		Warn.Println("RabbitMQ connection not established")
		return nil
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

	err = ch.QueueBind(
		queue.Name,
		queue.Name,
		"potat-api",
		false,
		nil,
	)
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

	go func() {
    for {
			select {
			case msg := <-msgs:
				// TODO: smarter way to ignore self messages uuh
				if strings.HasPrefix(string(msg.Body), "postgres") {
					msg.Reject(true)
				}
				if msg.Body != nil {
					msg.Ack(true)
					handleMessage(string(msg.Body))
				}
				msg.Nack(false, false)
			case msg := <-proxyMsgs:
				if msg.Body != nil && proxySocketFn != nil {
					msg.Ack(true)
					err := proxySocketFn(string(msg.Body))
					if err != nil {
						Warn.Printf("Failed to send message to socket: %v", err)
					}
				}
				msg.Nack(false, false)
			}
    }
	}()

	return nil
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

	queue, err := ch.QueueDeclare(
		"potat-api",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 5 * time.Second)
	defer cancel()

	err = ch.ExchangeDeclare(
		"potat-api",
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	err = ch.PublishWithContext(ctx,
		"potat-api",
		queue.Name,
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(message),
			Expiration:  fmt.Sprintf("%d", ttl.Milliseconds()),
		},
	)
	if err != nil {
		return err
	}

	Debug.Printf("[x] Sent %s", message)
	return nil
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
	case "postgres-backup":
		err := PublishToQueue(context.Background(), "backup", 5 * time.Second)
		if err != nil {
			Warn.Printf("Failed to send backup message: %v", err)
		}
	default:
		Debug.Printf("[x] Unrecognized topic: %s", topic)
	}
}
