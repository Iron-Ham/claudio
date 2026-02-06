package bridge

import (
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// defaultPollInterval is how often the monitor checks for instance completion.
const defaultPollInterval = time.Second

// Option configures a Bridge.
type Option func(*config)

type config struct {
	pollInterval time.Duration
	logger       *logging.Logger
}

// WithPollInterval sets the polling interval for completion checking.
// A zero or negative value is replaced with the default (1s).
func WithPollInterval(d time.Duration) Option {
	return func(c *config) {
		c.pollInterval = d
	}
}

// WithLogger sets the logger for the bridge.
func WithLogger(logger *logging.Logger) Option {
	return func(c *config) {
		c.logger = logger
	}
}
