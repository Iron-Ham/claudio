package pipeline

import (
	"github.com/Iron-Ham/claudio/internal/coordination"
	"github.com/Iron-Ham/claudio/internal/logging"
)

// PipelineOption configures a Pipeline.
type PipelineOption func(*pipelineConfig)

// WithHubOptions sets coordination.Hub options that are forwarded to every
// Manager created by the pipeline. Use this to configure scaling policies,
// instance counts, timing parameters, etc.
func WithHubOptions(opts ...coordination.Option) PipelineOption {
	return func(c *pipelineConfig) {
		c.hubOpts = append(c.hubOpts, opts...)
	}
}

// WithDebate enables the debate phase between execution and review.
// When enabled, the pipeline identifies tasks that touched overlapping files
// and creates structured debate sessions to reconcile potential conflicts
// before review.
func WithDebate() PipelineOption {
	return func(c *pipelineConfig) {
		c.enableDebate = true
	}
}

// WithLogger sets the logger for the pipeline. If not set, a NopLogger is used.
func WithLogger(l *logging.Logger) PipelineOption {
	return func(c *pipelineConfig) {
		c.logger = l
	}
}
