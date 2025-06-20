package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	Shortened   prometheus.Counter
	Unshortened prometheus.Counter
}

func New() *Metrics {
	return &Metrics{
		Shortened:   promauto.NewCounter(prometheus.CounterOpts{Name: "shortener_shortened_total"}),
		Unshortened: promauto.NewCounter(prometheus.CounterOpts{Name: "shortener_unshortened_total"}),
	}
}

func (m *Metrics) Shorten() {
	m.Shortened.Inc()
}

func (m *Metrics) Unshorten() {
	m.Unshortened.Inc()
}
