package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	reg              prometheus.Registerer
	tweets           *prometheus.CounterVec
	tweetsProcess    *prometheus.HistogramVec
	tweetsWrite      *prometheus.HistogramVec
	tweetsWriteBytes *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
	reg := prometheus.DefaultRegisterer
	factory := promauto.With(reg)

	metrics := &Metrics{
		reg: reg,
		tweets: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "tweets_total",
			Help: "Total number of tweets",
		}, []string{"filter"}),

		tweetsProcess: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name: "tweets_processing",
			Help: "Time for processing tweet, marshalling it to json and writing to Kafka",
		}, []string{"filter"}),

		tweetsWrite: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name: "tweets_write",
			Help: "Time for writing tweet to Kafka",
		}, []string{"filter"}),

		tweetsWriteBytes: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name: "tweets_write_bytes",
			Help: "Size in Bytes for what was written to Kafka",
		}, []string{"filter"}),
	}

	//metrics.reg.MustRegister(metrics.tweets)
	//metrics.reg.MustRegister(metrics.tweetsProcess)
	//metrics.reg.MustRegister(metrics.tweetsWrite)
	//metrics.reg.MustRegister(metrics.tweetsWriteBytes)
	return metrics
}
