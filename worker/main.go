package main

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/matheus-deoliveira/go-notifications-with-queue/events"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Simulador de Idempotência - Armazena IDs processados em memória
var processedEvents = make(map[string]bool)

func failOnErrorReceive(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnErrorReceive(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnErrorReceive(err, "Failed to open a channel")
	defer ch.Close()

	// 1. Declarar a Dead Letter Exchange (DLX) e DLQ antes
	err = ch.ExchangeDeclare(
		"dlx_credit_events",
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	failOnErrorReceive(err, "Failed to declare DLX")

	dlq, err := ch.QueueDeclare(
		"credit_approved_dlq",
		true,
		false,
		false,
		false,
		nil,
	)
	failOnErrorReceive(err, "Failed to declare DLQ")

	err = ch.QueueBind(
		dlq.Name,
		"credit_approved_dlq", // routing key da DLQ
		"dlx_credit_events",
		false,
		nil,
	)
	failOnErrorReceive(err, "Failed to bind DLQ")

	// 2. Declarar Exchange Principal e Fila Principal no Worker também
	err = ch.ExchangeDeclare(
		"credit_events", // name
		"direct",        // type
		true,            // durable
		false,           // auto-deleted
		false,           // internal
		false,           // no-wait
		nil,             // arguments
	)
	failOnErrorReceive(err, "Failed to declare an exchange")

	args := amqp.Table{
		"x-dead-letter-exchange":    "dlx_credit_events",
		"x-dead-letter-routing-key": "credit_approved_dlq",
	}

	q, err := ch.QueueDeclare(
		"credit_approved_queue",
		true,
		false,
		false,
		false,
		args,
	)
	failOnErrorReceive(err, "Failed to declare queue")

	err = ch.QueueBind(
		q.Name,
		"credit_approved",
		"credit_events",
		false,
		nil,
	)
	failOnErrorReceive(err, "Failed to bind queue")

	msgs, err := ch.Consume(
		"credit_approved_queue", // queue
		"",                      // consumer
		false,                   // auto-ack (False = vamos controlar o Ack manualmente)
		false,                   // exclusive
		false,                   // no-local
		false,                   // no-wait
		nil,                     // args
	)
	failOnErrorReceive(err, "Failed to register a consumer")

	var forever chan struct{}

	go func() {
		for d := range msgs {
			log.Printf("Received a message with ID: %s", d.MessageId)
			processMessage(ch, d)
		}
	}()

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever
}

func processMessage(ch *amqp.Channel, d amqp.Delivery) {
	var event events.CreditApprovedEvent
	err := json.Unmarshal(d.Body, &event)
	if err != nil {
		log.Printf("Unprocessable entity, rejecting... %v", err)
		d.Nack(false, false)
		return
	}

	if processedEvents[event.EventId] {
		log.Printf("[IDEMPOTÊNCIA] Evento %s já foi processado anteriormente. Ignorando.", event.EventId)
		d.Ack(false)
		return
	}

	err = sendEmail(event)
	if err != nil {
		log.Printf("Falha ao enviar e-mail: %v", err)
		handleFailure(ch, d, event)
		return
	}

	processedEvents[event.EventId] = true
	log.Printf("Sucesso! E-mail de confirmação enviado para %s", event.CustomerEmail)
	d.Ack(false)
}

func sendEmail(event events.CreditApprovedEvent) error {
	log.Printf("Enviando contrato limit %.2f para e-mail %s", event.CreditLimit, event.CustomerEmail)
	return errors.New("timeout na API Sendgrid (simulado)")
}

func handleFailure(ch *amqp.Channel, d amqp.Delivery, event events.CreditApprovedEvent) {
	retryCount := int32(0)
	if count, ok := d.Headers["x-retry-count"].(int32); ok {
		retryCount = count
	}

	if retryCount >= 3 {
		log.Printf("Limite de tentativas (3) atingido para evento %s. Enviando para DLQ.", event.EventId)
		d.Nack(false, false)
		return
	}

	retryCount++
	log.Printf("Tentativa de envio falhou. Agendando retry #%d", retryCount)

	headers := d.Headers
	if headers == nil {
		headers = make(amqp.Table)
	}
	headers["x-retry-count"] = retryCount

	err := ch.Publish(
		"credit_events",
		"credit_approved",
		false,
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         d.Body,
			Headers:      headers,
		})

	if err != nil {
		log.Printf("Falha ao republicar mensagem de retry: %v", err)
		d.Nack(false, true)
		return
	}

	d.Ack(false)
}
