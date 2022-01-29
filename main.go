package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"

	"github.com/Shopify/sarama"
)

var debug bool
var buildNum string

// server - Configuration for Service - I don't like keys/tokens in server struct.server.server.server
type server struct {
	log         *log.Entry
	kafkaClient sarama.SyncProducer
	kafkaTopic  string
	metrics     *Metrics
}

func getTwitterClient(ctx context.Context) (*http.Client, error) {
	consumerKey := os.Getenv("CONSUMER_KEY")
	consumerSecret := os.Getenv("CONSUMER_SECRET")
	accessToken := os.Getenv("ACCESS_TOKEN")
	accessSecret := os.Getenv("ACCESS_SECRET")

	if consumerKey == "" || consumerSecret == "" || accessToken == "" || accessSecret == "" {
		return &http.Client{}, errors.New("Access Key environment Variables missing")
	}

	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessSecret)

	httpClient := config.Client(ctx, token)

	return httpClient, nil
}

// NewServer - Create Server instance with Logger and Kafka Client
func NewServer(brokers, topic *string) (*server, error) {
	if debug {
		log.SetFormatter(&log.TextFormatter{})
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetFormatter(&log.JSONFormatter{})
	}

	logger := log.WithFields(log.Fields{
		"service": "feedster",
		"build":   buildNum,
	})

	b := *brokers
	t := *topic

	// Setup Sarama client for kafka
	config := sarama.NewConfig()
	config.Producer.Partitioner = sarama.NewRandomPartitioner
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 3
	config.Producer.Return.Successes = true
	client, err := sarama.NewSyncProducer(strings.Split(b, ","), config)
	if err != nil {
		return &server{}, fmt.Errorf("Error with Kafka Client: %v", err)
	}

	s := &server{
		log:         logger,
		kafkaClient: client,
		kafkaTopic:  t,
		metrics:     NewMetrics(),
	}

	return s, nil
}

func main() {
	var port = "5000"

	flag.StringVar(&port, "port", ":5000", "Port")
	flag.BoolVar(&debug, "debug", false, "Debug Logging")
	trackVar := flag.String("track", "", "Comma separated list of words/hashtags to track on twitter")
	brokers := flag.String("brokers", "", "Comma separated list of Kafka brokers to connect too")
	topic := flag.String("topic", "feedster-sink", "Kafka Topic to write too")
	flag.Parse()

	// Setup Connection timeouts
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	// Setup Server
	server, err := NewServer(brokers, topic)
	if err != nil {
		log.Info("Error Setting Up Server: ", err)
		os.Exit(1)
	}
	defer server.kafkaClient.Close()

	server.log.Info("Starting Server...")
	server.log.WithFields(log.Fields{
		"brokers": brokers,
	}).Info("Connecting To Brokers")

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	defer func() {
		signal.Stop(signalChan)
		cancel()
	}()

	go func() {
		select {
		case <-signalChan:
			server.log.Info("Shutting Down")
			cancel()
		case <-ctx.Done():
			server.log.Info("Context Shutting Down")
		}
		os.Exit(1)
	}()

	// Process trackVars to list
	t := *trackVar
	track := strings.Split(t, ",")

	server.log.Info("Starting Twitter Stream")
	go server.run(ctx, track)

	// Start HTTP Server for metrics/status
	http.HandleFunc("/", handler) // http://127.0.0.1:8080/Go
	http.Handle("/metrics", promhttp.Handler())

	server.log.Info("Starting Web Server on port: ", port)
	server.log.Fatal(http.ListenAndServe(port, nil))
}

func (s *server) run(ctx context.Context, track []string) {
	// oAuth http client
	httpClient, err := getTwitterClient(ctx)
	if err != nil {
		// I should fail a health check here
		// either crash the app, or log loop with error
		s.log.Error("Error Creating Twitter HTTP Client: ", err.Error())
		return
	}

	// Twitter Client
	client := twitter.NewClient(httpClient)

	params := &twitter.StreamFilterParams{
		Track:         track,
		StallWarnings: twitter.Bool(true),
	}

	// Start Stream
	stream, err := client.Streams.Filter(params)
	if err != nil {
		s.log.Error("Unable to Connect to Twitter Stream: %v", err)
	}

	s.log.Debug("Conntected to Twitter, Streaming Messages...")
	// Iterate on messages, catch cancel callout
	for message := range stream.Messages {
		// Start timer for Marshalling
		start := time.Now()
		// Probably dont write the whole json?
		data, _ := json.Marshal(message)
		message := &sarama.ProducerMessage{
			Topic:     s.kafkaTopic,
			Partition: -1,
			Value:     sarama.StringEncoder(data),
		}
		partition, offset, err := s.kafkaClient.SendMessage(message)
		s.metrics.tweetsProcess.WithLabelValues(strings.Join(track, ",")).Observe(float64(time.Since(start).Nanoseconds()))
		s.metrics.tweetsWriteBytes.WithLabelValues(strings.Join(track, ",")).Observe(float64(len(data)))
		logger := s.log.WithFields(log.Fields{
			"topic":     s.kafkaTopic,
			"partition": partition,
			"offset":    offset,
		})
		if err != nil {
			logger.WithFields(log.Fields{
				"status": "error",
			}).Info("Error Writing to Kafka", err)
		} else {
			s.metrics.tweets.WithLabelValues(strings.Join(track, ",")).Inc()
			logger.WithFields(log.Fields{
				"status": "success",
			}).Debug("Added Tweet to Kafka")
		}
	}
}

func tweetPayload(data interface{}) twitter.Tweet {
	tweet := twitter.Tweet{}
	bodyBytes, _ := json.Marshal(data)
	json.Unmarshal(bodyBytes, &tweet)
	return tweet
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hi there, I love %s!", r.URL.Path[1:])
}
