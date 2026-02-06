package pipeline

import "github.com/Iron-Ham/claudio/internal/coordination"

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
