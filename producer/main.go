package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/matheus-deoliveira/go-notifications-with-queue/events"
	amqp "github.com/rabbitmq/amqp091-go"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	// Declarar Exchange Principal
	err = ch.ExchangeDeclare(
		"credit_events", // name
		"direct",        // type
		true,            // durable
		false,           // auto-deleted
		false,           // internal
		false,           // no-wait
		nil,             // arguments
	)
	failOnError(err, "Failed to declare an exchange")

	// Configurar argumentos para a fila principal (Roteamento para DLQ)
	args := amqp.Table{
		"x-dead-letter-exchange":    "dlx_credit_events",   // Envia mensagens mortas para esta exchange
		"x-dead-letter-routing-key": "credit_approved_dlq", // Com esta routing key
	}

	q, err := ch.QueueDeclare(
		"credit_approved_queue", // name
		true,                    // durable
		false,                   // delete when unused
		false,                   // exclusive
		false,                   // no-wait
		args,                    // arguments
	)
	failOnError(err, "Failed to declare a queue")

	err = ch.QueueBind(
		q.Name,            // queue name
		"credit_approved", // routing key
		"credit_events",   // exchange
		false,
		nil,
	)
	failOnError(err, "Failed to bind a queue")

	// Prepara evento fake utilizando o pacote compartilhado
	event := events.CreditApprovedEvent{
		EventId:       "evt-12345",
		ProposalId:    "prop-9876",
		ApprovedAt:    time.Now(),
		CustomerName:  "João Silva",
		CustomerEmail: "joao.silva@example.com",
		CreditLimit:   50000.00,
	}

	body, err := json.Marshal(event)
	failOnError(err, "Failed to marshal JSON")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = ch.PublishWithContext(ctx,
		"credit_events",   // exchange
		"credit_approved", // routing key
		false,             // mandatory
		false,             // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
			Headers: amqp.Table{
				"x-retry-count": int32(0),
			},
		})
	failOnError(err, "Failed to publish a message")

	log.Printf(" [x] Sent Credit Approval Event: %s", event.EventId)
}
