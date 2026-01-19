// Package prbuilder generates pull request content from completed tasks.
// It handles title generation, body formatting, and metadata aggregation
// without performing any git operations or I/O.
package prbuilder

import (
	"github.com/Iron-Ham/claudio/internal/orchestrator/consolidation"
)

// Compile-time check that Builder implements consolidation.PRBuilder.
var _ consolidation.PRBuilder = (*Builder)(nil)

// Builder generates PR content from completed tasks.
type Builder struct {
	objectiveLimit int
}

// Option configures a Builder.
type Option func(*Builder)

// WithObjectiveLimit sets the maximum length for objective text in titles.
func WithObjectiveLimit(limit int) Option {
	return func(b *Builder) {
		b.objectiveLimit = limit
	}
}

// New creates a new Builder with the given options.
func New(opts ...Option) *Builder {
	b := &Builder{
		objectiveLimit: 50,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Build generates complete PR content from the given tasks and options.
func (b *Builder) Build(tasks []consolidation.CompletedTask, opts consolidation.PRBuildOptions) (*consolidation.PRContent, error) {
	return &consolidation.PRContent{
		Title:      b.BuildTitle(tasks, opts),
		Body:       b.BuildBody(tasks, opts),
		Labels:     b.BuildLabels(tasks, opts),
		BaseBranch: opts.BaseBranch,
		HeadBranch: opts.HeadBranch,
	}, nil
}

// BuildTitle generates a PR title from tasks and options.
func (b *Builder) BuildTitle(tasks []consolidation.CompletedTask, opts consolidation.PRBuildOptions) string {
	return buildTitle(opts.Objective, opts.Mode, opts.GroupIndex, b.objectiveLimit)
}

// BuildBody generates a PR body from tasks and options.
func (b *Builder) BuildBody(tasks []consolidation.CompletedTask, opts consolidation.PRBuildOptions) string {
	return buildBody(tasks, opts)
}

// BuildLabels determines appropriate labels for the PR.
func (b *Builder) BuildLabels(tasks []consolidation.CompletedTask, opts consolidation.PRBuildOptions) []string {
	return buildLabels(tasks)
}
