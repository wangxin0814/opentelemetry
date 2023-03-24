package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/wangxin0814/opentelemetry/kafka"
	"go.opentelemetry.io/contrib/instrumentation/github.com/Shopify/sarama/otelsarama"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"log"
	"os"
	"strings"
)

var (
	brokers = flag.String("brokers", os.Getenv("KAFKA_PEERS"), "The Kafka brokers to connect to, as a comma separated list")
)

func main() {
	//os.Setenv("KAFKA_PEERS", "192.168.0.120:9092,192.168.0.120:9093,192.168.0.120:9094")

	tp, err := kafka.InitTracer("producer")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()
	flag.Parse()

	if *brokers == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	brokerList := strings.Split(*brokers, ",")
	log.Printf("Kafka brokers: %s", strings.Join(brokerList, ", "))

	producer, err := newAccessLogProducer(brokerList)
	if err != nil {
		log.Fatal(err)
	}

	// Create root span
	//tr := otel.Tracer("producer")
	ctx := context.Background()
	var (
		span trace.Span
	)

	// root span
	tracer := kafka.TraceFromContext(ctx)
	ctx, span = tracer.Start(ctx, "produce message")
	defer span.End()

	// Inject tracing info into message
	//rng := rand.New(rand.NewSource(time.Now().Unix()))
	msg := sarama.ProducerMessage{
		Topic: kafka.KafkaTopic,
		Key:   sarama.StringEncoder("random_number"),
		//Value: sarama.StringEncoder(fmt.Sprintf("%d", rng.Intn(1000))),
		Value: sarama.StringEncoder("hello"),
	}
	otel.GetTextMapPropagator().Inject(ctx, otelsarama.NewProducerMessageCarrier(&msg))

	producer.Input() <- &msg
	successMsg := <-producer.Successes()
	log.Println("Successful to write message, offset:", successMsg.Offset)

	err = producer.Close()
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		log.Fatalln("Failed to close producer:", err)
	}
}

func newAccessLogProducer(brokerList []string) (sarama.AsyncProducer, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V2_5_0_0
	// So we can know the partition and offset of messages.
	config.Producer.Return.Successes = true

	producer, err := sarama.NewAsyncProducer(brokerList, config)
	if err != nil {
		return nil, fmt.Errorf("starting Sarama producer: %w", err)
	}

	// Wrap instrumentation
	producer = otelsarama.WrapAsyncProducer(config, producer)

	// We will log to STDOUT if we're not able to produce messages.
	go func() {
		for err := range producer.Errors() {
			log.Println("Failed to write message:", err)
		}
	}()

	return producer, nil
}
