// Package streamjson provides types and parsers for Claude Code's stream-json
// output format. When invoked with `claude -p --output-format stream-json`,
// Claude Code emits newline-delimited JSON (NDJSON) events to stdout.
//
// This package is the foundation for subprocess-based instance execution,
// which replaces tmux-based screen scraping with direct process control
// and structured event parsing.
//
// Usage:
//
//	r := streamjson.NewReader(stdout)
//	for {
//	    event, err := r.Next()
//	    if err == io.EOF { break }
//	    switch e := event.(type) {
//	    case *streamjson.SystemEvent:
//	        // initialization data
//	    case *streamjson.AssistantEvent:
//	        // model output
//	    case *streamjson.ResultEvent:
//	        // final result with usage/cost
//	    }
//	}
package streamjson
