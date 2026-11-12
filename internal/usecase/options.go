package usecase

import "time"

type options struct {
	now func() time.Time
}

type Option func(*options)

// WithClock позволяет задавать время в тестах
func WithClock(now func() time.Time) Option {
	return func(o *options) {
		if now != nil {
			o.now = now
		}
	}
}

// ResolveClock возвращает настроенные часы или time
func ResolveClock(opts ...Option) func() time.Time {
	o := options{now: time.Now}
	for _, option := range opts {
		if option != nil {
			option(&o)
		}
	}
	return o.now
}
