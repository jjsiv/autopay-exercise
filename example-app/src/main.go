package main

import (
	"context"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/spf13/cobra"
)

var (
	flagTopic   string
	flagBrokers []string
)

func runSubscriber() *cobra.Command {
	command := &cobra.Command{
		Use:   "sub",
		Short: "Run Kafka subscriber",
		Run: func(cmd *cobra.Command, args []string) {
			log.Println("Starting Kafka subscriber...")

			r := kafka.NewReader(kafka.ReaderConfig{
				Brokers: flagBrokers,
				Topic:   flagTopic,
			})

			for {
				m, err := r.ReadMessage(context.Background())
				if err != nil {
					log.Printf("could not read message: %v", err)
					continue
				}
				log.Printf("message at offset %d: %s = %s\n", m.Offset, string(m.Key), string(m.Value))
			}
		},
	}
	command.Flags().StringVarP(&flagTopic, "topic", "t", "", "Kafka topic name")
	command.Flags().StringArrayVarP(&flagBrokers, "brokers", "b", []string{}, "Broker address, can be set multiple times")

	return command
}

func runPublisher() *cobra.Command {
	command := &cobra.Command{
		Use:   "pub",
		Short: "Run Kafka publisher",
		Run: func(cmd *cobra.Command, args []string) {
			log.Println("Starting Kafka publisher...")

			w := &kafka.Writer{
				Addr:  kafka.TCP(flagBrokers...),
				Topic: flagTopic,
			}

			messages := []kafka.Message{
				{
					Key:   []byte("testA"),
					Value: []byte("Hello World!"),
				},
			}

			for {
				ctx := context.Background()
				if err := w.WriteMessages(ctx, messages...); err != nil {
					log.Printf("unexpected error %v", err)
				}

				log.Println("sleeping for 5s...")
				time.Sleep(time.Second * 5)
			}
		},
	}

	command.Flags().StringVarP(&flagTopic, "topic", "t", "", "Kafka topic name")
	command.Flags().StringArrayVarP(&flagBrokers, "brokers", "b", []string{}, "Broker address, can be set multiple times")

	return command
}

func main() {
	cmd := &cobra.Command{
		Use:   "example-app",
		Short: "Example app for testing topics",
	}
	cmd.AddCommand(
		runPublisher(),
		runSubscriber(),
	)
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
