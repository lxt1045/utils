package cache

import (
	"log/slog"
)

type config[T Value] struct {
	Name         string
	MetricSec    int
	MetricLogger func(attrs ...slog.Attr)
	BatchLoader  BatchLoadFunc[T]
	PostLoad     PostLoadFunc[T]
}
type Option[T Value] func(*config[T])

func WithMetrics[T Value](name string, sec int, logger func(attrs ...slog.Attr)) Option[T] {
	return func(c *config[T]) {
		if sec == 0 {
			sec = 600
		}
		c.Name = name
		c.MetricSec = sec
		c.MetricLogger = logger
	}
}

func WithBatchLoad[T Value](f BatchLoadFunc[T]) Option[T] {
	return func(c *config[T]) {
		c.BatchLoader = f
	}
}
func WithPostLoad[T Value](f PostLoadFunc[T]) Option[T] {
	return func(c *config[T]) {
		c.PostLoad = f
	}
}
