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

func CreateBroker(config common.Config) error {
	connString := fmt.Sprintf(
		"amqp://%s:%s@%s:%s/",
		config.RabbitMQ.User,
		config.RabbitMQ.Password,
		config.RabbitMQ.Host,
		config.RabbitMQ.Port,
	)

  var err error
  Conn, err = amqp.Dial(connString)
	if err != nil {
    return err
  }

	Info.Printf("Connected to RabbitMQ")

	// send example test message
	err = PublishToQueue(context.Background(), "test message")
	if err != nil {
		return err
	}

	return nil
}

func Stop() {
	if Conn != nil {
		_ = Conn.Close()
		Info.Printf("RabbitMQ connection closed")
	}
}

func PublishToQueue(
	ctx context.Context,
	message string,
) error {
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
		},
	)
	if err != nil {
		return err
	}

	Debug.Printf(" [x] Sent %s", message)
	return nil
}