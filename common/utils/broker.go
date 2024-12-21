package utils

import (
	"context"
	"fmt"
	"potat-api/common"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	Conn *amqp.Connection
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

func Stop() {
	if Conn != nil {
		_ = Conn.Close()
		Warn.Printf("RabbitMQ connection closed")
	}
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

	msgs, err := ch.ConsumeWithContext(
		ctx,
		queue.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	go func() {
		for d := range msgs {
			Debug.Printf("[x] Received %s", d.Body)
			handleMessage(string(d.Body))
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
	switch message {
	case "shutdown":
		break; // Do nothing for now :)
	case "ping":
	  PublishToQueue(context.Background(), "pong", 5 * time.Second)
	}
}
		