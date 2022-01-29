package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	reg              *prometheus.Registry
	tweets           *prometheus.CounterVec
	tweetsProcess    *prometheus.HistogramVec
	tweetsWrite      *prometheus.HistogramVec
	tweetsWriteBytes *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
	namespace := "feedster"

	reg := prometheus.NewRegistry()
	factory := promauto.With(reg)

	return &Metrics{
		reg: reg,

		tweets: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "tweets_total",
			Help:      "Total number of tweets processed by process",
		}, []string{"tracks"}),

		tweetsProcess: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "tweets_processing",
			Help:      "Time for processing tweet, marshalling it to json for writing to Kafka",
		}, []string{"tracks"}),

		tweetsWrite: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "tweets_write",
			Help:      "Time for writing tweet to Kafka",
		}, []string{"tracks"}),

		tweetsWriteBytes: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "tweets_write_bytes",
			Help:      "Size in Bytes for what was written to Kafka",
		}, []string{"tracks"}),
	}
}
