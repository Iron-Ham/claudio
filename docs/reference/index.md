# Reference

Technical reference documentation for Claudio.

## Reference Guides

### [CLI Reference](cli.md)
Complete documentation for all Claudio commands, flags, and options.

### [Configuration Reference](configuration.md)
All configuration options with types, defaults, and examples.

### [Keyboard Shortcuts](keyboard-shortcuts.md)
Quick reference card for TUI keyboard shortcuts.

## Quick Links

- [User Guide](../guide/index.md) - Conceptual documentation
- [Tutorials](../tutorials/index.md) - Step-by-step guides
- [Troubleshooting](../troubleshooting.md) - Common issues
- [FAQ](../faq.md) - Frequently asked questions

## Command Quick Reference

| Command | Description |
|---------|-------------|
| `claudio init` | Initialize in git repo |
| `claudio start [name]` | Start session with TUI |
| `claudio add "task"` | Add new instance |
| `claudio status` | Show session status |
| `claudio stop` | Stop all instances |
| `claudio pr [id]` | Create pull request |
| `claudio config` | Configure Claudio |
| `claudio cleanup` | Clean stale resources |
| `claudio harvest` | Review completed work |
| `claudio remove <id>` | Remove instance |
| `claudio stats` | Show resource usage |
| `claudio sessions` | Manage sessions |
| `claudio ultraplan` | Orchestrated multi-task execution |

## Review Workflow Quick Reference

For parallel code review with specialized agents:

```bash
# Start review session
claudio start review-session

# Add specialized reviewers
claudio add "Security review: OWASP Top 10 vulnerabilities"
claudio add "Performance review: N+1 queries, bottlenecks"
claudio add "Style review: coding standards compliance"

# Monitor and export
claudio status --verbose > review-results.txt
```

See [Code Review Workflow Tutorial](../tutorials/code-review-workflow.md) for details.

## Environment Variables

All config options can be set via environment:

```bash
CLAUDIO_COMPLETION_DEFAULT_ACTION=auto_pr
CLAUDIO_BRANCH_PREFIX=feature
CLAUDIO_PR_DRAFT=true
CLAUDIO_RESOURCES_COST_LIMIT=10.00
```

See [Configuration Reference](configuration.md) for complete list.
