package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	MessagesProcessed prometheus.Counter
	MessagesDropped   prometheus.Counter
	MessagesSentToDLQ prometheus.Counter
	TransformDuration *prometheus.HistogramVec
	TransformErrors   *prometheus.CounterVec
	PipelineDuration  prometheus.Histogram
}

func New() *Metrics {
	return NewWithNamespace("", "", nil)
}

func NewWithNamespace(namespace, subsystem string, buckets map[string][]float64) *Metrics {
	return &Metrics{
		MessagesProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "transform_messages_processed_total",
			Help:      "Total number of messages processed",
		}),
		MessagesDropped: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "transform_messages_dropped_total",
			Help:      "Messages discarded by filters or validation",
		}),
		MessagesSentToDLQ: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "transform_messages_dlq_total",
			Help:      "Messages sent to the dead letter queue",
		}),
		TransformDuration: promauto.NewHistogramVec(transformHistogramOpts(namespace, subsystem, buckets), []string{"transform", "pipeline"}),
		TransformErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "transform_errors_total",
			Help:      "Errors surfaced by transforms",
		}, []string{"transform", "pipeline"}),
		PipelineDuration: promauto.NewHistogram(pipelineHistogramOpts(namespace, subsystem, buckets)),
	}
}

func transformHistogramOpts(namespace, subsystem string, buckets map[string][]float64) prometheus.HistogramOpts {
	opts := prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "transform_duration_seconds",
		Help:      "Duration of each transform stage",
	}
	if custom := customBuckets(buckets, "transform_latency"); len(custom) > 0 {
		opts.Buckets = custom
	} else if custom := customBuckets(buckets, "latency"); len(custom) > 0 {
		opts.Buckets = custom
	} else {
		opts.Buckets = prometheus.ExponentialBuckets(0.001, 2, 10)
	}
	return opts
}

func pipelineHistogramOpts(namespace, subsystem string, buckets map[string][]float64) prometheus.HistogramOpts {
	opts := prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "pipeline_duration_seconds",
		Help:      "End-to-end pipeline duration",
	}
	if custom := customBuckets(buckets, "pipeline_latency"); len(custom) > 0 {
		opts.Buckets = custom
	} else if custom := customBuckets(buckets, "latency"); len(custom) > 0 {
		opts.Buckets = custom
	} else {
		opts.Buckets = prometheus.ExponentialBuckets(0.001, 2, 10)
	}
	return opts
}

func customBuckets(buckets map[string][]float64, key string) []float64 {
	if buckets == nil {
		return nil
	}
	if vals, ok := buckets[key]; ok {
		return vals
	}
	return nil
}
