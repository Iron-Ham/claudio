// Package detect provides output analysis for detecting Claude Code's waiting states.
// It analyzes terminal output to determine whether Claude is actively working,
// waiting for user input, asking questions, or has completed/errored.
package detect

import (
	"regexp"
	"strings"
)

// WaitingState represents different types of waiting conditions Claude can be in.
// The state machine prioritizes working indicators over historical questions,
// ensuring that recent activity is properly detected even if earlier output
// contained questions or prompts.
type WaitingState int

const (
	// StateWorking means Claude is actively working (not waiting).
	// This is the default state when no other patterns match, and it's also
	// returned when working indicators (like "Reading...", spinner chars)
	// are detected, even if there are questions in the output history.
	StateWorking WaitingState = iota

	// StateWaitingPermission means Claude is asking for permission to perform an action.
	// This includes Y/N prompts, "Shall I proceed?" questions, and explicit
	// permission requests. This is the highest-priority waiting state.
	StateWaitingPermission

	// StateWaitingQuestion means Claude is asking the user a question.
	// This includes questions ending with "?", "please specify", "select one",
	// and other information-gathering prompts.
	StateWaitingQuestion

	// StateWaitingInput means Claude is waiting for general input.
	// This detects Claude Code's UI elements like the input prompt,
	// bypass mode indicators, and send buttons.
	StateWaitingInput

	// StateCompleted means Claude has finished its task.
	// NOTE: For Claudio orchestration, completion is primarily detected via
	// sentinel files (.claudio-task-complete.json), not text patterns.
	// Text-based completion detection is intentionally disabled to avoid
	// false positives.
	StateCompleted

	// StateError means Claude encountered a critical error that stopped execution.
	// This only matches Claude CLI-specific errors (session/connection failures,
	// rate limits, signal termination) - not general error text in command output.
	StateError

	// StatePROpened means Claude opened a pull request (PR URL detected in output).
	// This allows orchestration to react when a PR is successfully created.
	StatePROpened
)

// String returns a human-readable string for the waiting state.
func (s WaitingState) String() string {
	switch s {
	case StateWorking:
		return "working"
	case StateWaitingPermission:
		return "waiting_permission"
	case StateWaitingQuestion:
		return "waiting_question"
	case StateWaitingInput:
		return "waiting_input"
	case StateCompleted:
		return "completed"
	case StateError:
		return "error"
	case StatePROpened:
		return "pr_opened"
	default:
		return "unknown"
	}
}

// IsWaiting returns true if the state represents any waiting condition
// (permission, question, or general input).
func (s WaitingState) IsWaiting() bool {
	return s == StateWaitingPermission || s == StateWaitingQuestion || s == StateWaitingInput
}

// StateDetector analyzes Claude's output to determine its current waiting state.
// Implementations should be thread-safe for concurrent use.
type StateDetector interface {
	// Detect analyzes output and returns the detected waiting state.
	// It examines the last portion of output (most recent content) for patterns.
	// Empty or nil output returns StateWorking.
	Detect(output []byte) WaitingState
}

// Pattern categories for state detection.
// Each category groups regex patterns that identify a specific state.
var (
	// PermissionPatterns detect Claude asking for permission to perform actions.
	// These typically appear with Yes/No options or require explicit approval.
	PermissionPatterns = []string{
		// Standard permission prompts
		`(?i)do you want (?:me )?to (?:proceed|continue|run|execute|apply|make)`,
		`(?i)(?:shall|should|can|may) I (?:proceed|continue|go ahead|run|execute|apply)`,
		`(?i)(?:allow|permit|approve) (?:this|the) (?:action|change|operation)`,
		`(?i)\[Y(?:es)?/[Nn](?:o)?\]`,                                    // [Y/N] or [Yes/No] prompts
		`(?i)\(y(?:es)?/n(?:o)?\)`,                                       // (y/n) or (yes/no) prompts
		`(?i)press (?:y|enter) to (?:confirm|continue|proceed|approve)`,  // Press y to confirm
		`(?i)type ['"]?(?:yes|y)['"]? to (?:confirm|continue|proceed)`,   // Type yes to confirm
		`(?i)waiting for (?:your )?(?:approval|confirmation|permission)`, // Explicit waiting
		`(?i)requires? (?:your )?(?:approval|confirmation|permission)`,   // Requires permission
	}

	// QuestionPatterns detect Claude asking for information or clarification.
	QuestionPatterns = []string{
		// Direct questions at end of output (question mark at end of recent line)
		`\?\s*$`,
		// Explicit question phrases
		`(?i)(?:what|which|how|where|when|who|why) (?:would you|do you|should I|is the)`,
		`(?i)(?:can|could|would) you (?:tell me|specify|clarify|explain|provide)`,
		`(?i)please (?:specify|clarify|provide|tell me|let me know)`,
		`(?i)I need (?:to know|more information|clarification|you to)`,
		`(?i)(?:select|choose|pick) (?:one|an option|from)`,
		// Waiting for specific input
		`(?i)waiting for (?:your )?(?:input|response|answer|reply)`,
		`(?i)enter (?:your|the|a) `,
	}

	// InputWaitingPatterns detect Claude Code's UI elements indicating it's waiting.
	// These are specific to Claude Code's terminal interface.
	InputWaitingPatterns = []string{
		// Claude Code prompt mode indicators (shown in status bar)
		`⏵⏵\s*bypass permissions`,      // Bypass mode indicator
		`⏵\s*(?:allow|approve|bypass)`, // Single arrow prompt indicators
		`↵\s*send`,                     // Send indicator at end of input line
		`\(shift\+tab to cycle\)`,      // Mode cycling hint
		// Input prompt line pattern (> followed by text and send indicator)
		`>\s+.*↵`,
	}

	// CompletionPatterns detect task completion.
	// NOTE: Intentionally empty - completion is detected via sentinel files,
	// not text patterns. See StateCompleted documentation.
	CompletionPatterns = []string{}

	// ErrorPatterns detect critical Claude CLI errors that stopped execution.
	// These are specific to actual Claude failures, not error text in command output.
	ErrorPatterns = []string{
		// Claude CLI specific error messages
		`(?i)^Error: (?:session|connection|authentication|api) `,
		`(?i)claude (?:exited|terminated|crashed|died) (?:with|unexpectedly)`,
		// Process termination signals
		`(?i)(?:signal|killed|terminated): (?:SIGTERM|SIGKILL|SIGINT)`,
		// Rate limiting or API errors from Claude
		`(?i)(?:rate limit|quota) (?:exceeded|reached)`,
		`(?i)(?:api|request) (?:error|failed).*(?:401|403|429|500|502|503)`,
	}

	// WorkingPatterns detect Claude actively working (override waiting detection).
	// When these match recent output, working state is returned even if there
	// are questions in the output history.
	WorkingPatterns = []string{
		`(?i)(?:reading|writing|editing|creating|modifying|analyzing|searching|running|executing|building|compiling|testing)\.{3}`,
		`(?i)(?:let me (?:check|look|see|analyze|examine|investigate|read|search|find)|i'?ll (?:check|look|start|begin)|going to (?:check|look|analyze|implement)|about to (?:start|begin|run))`,
		`(?i)(?:working on|processing|loading|fetching)`,
		`⠋|⠙|⠹|⠸|⠼|⠴|⠦|⠧|⠇|⠏`, // Spinner characters
	}

	// PROpenedPatterns detect when a pull request URL appears in output.
	// This indicates Claude has successfully created a PR via gh pr create.
	PROpenedPatterns = []string{
		// GitHub PR URL pattern: https://github.com/owner/repo/pull/123
		`https://github\.com/[^/]+/[^/]+/pull/\d+`,
	}
)

// Detector implements StateDetector using regex pattern matching.
// It maintains compiled regex patterns for efficiency and analyzes
// the most recent portion of output to determine Claude's state.
type Detector struct {
	permissionPatterns   []*regexp.Regexp
	questionPatterns     []*regexp.Regexp
	inputWaitingPatterns []*regexp.Regexp
	completionPatterns   []*regexp.Regexp
	errorPatterns        []*regexp.Regexp
	workingPatterns      []*regexp.Regexp
	prOpenedPatterns     []*regexp.Regexp
}

// NewDetector creates a new output state detector with pre-compiled regex patterns.
func NewDetector() *Detector {
	return &Detector{
		permissionPatterns:   compilePatterns(PermissionPatterns),
		questionPatterns:     compilePatterns(QuestionPatterns),
		inputWaitingPatterns: compilePatterns(InputWaitingPatterns),
		completionPatterns:   compilePatterns(CompletionPatterns),
		errorPatterns:        compilePatterns(ErrorPatterns),
		workingPatterns:      compilePatterns(WorkingPatterns),
		prOpenedPatterns:     compilePatterns(PROpenedPatterns),
	}
}

// compilePatterns compiles a list of regex pattern strings.
// Invalid patterns are silently skipped.
func compilePatterns(patterns []string) []*regexp.Regexp {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}

// Detect analyzes output and returns the detected waiting state.
// It examines the last portion of output (last ~2000 chars) for patterns.
//
// Detection priority (highest to lowest):
//  1. Working indicators - if Claude is actively working, return StateWorking
//  2. PR opened - if a GitHub PR URL is found, return StatePROpened
//  3. Errors - if critical Claude CLI errors are found, return StateError
//  4. Completion - if completion patterns match (currently disabled), return StateCompleted
//  5. Permission prompts - if Y/N or permission requests found, return StateWaitingPermission
//  6. Questions - if questions are found, return StateWaitingQuestion
//  7. Input prompts - if Claude Code UI elements detected, return StateWaitingInput
//  8. Default - return StateWorking
func (d *Detector) Detect(output []byte) WaitingState {
	if len(output) == 0 {
		return StateWorking
	}

	// Focus on the last portion of output (last ~2000 chars for efficiency)
	text := string(output)
	if len(text) > 2000 {
		text = text[len(text)-2000:]
	}

	// Strip ANSI escape codes for cleaner pattern matching
	text = StripAnsi(text)

	// Get the last few lines for more focused analysis
	lines := strings.Split(text, "\n")
	recentLines := getLastNonEmptyLines(lines, 10)
	recentText := strings.Join(recentLines, "\n")

	// Check for active working indicators first - if Claude is actively working,
	// don't report as waiting even if there's a question in the output history
	if d.matchesAny(recentText, d.workingPatterns) {
		return StateWorking
	}

	// Check for PR opened (highest priority - PR URL in output means work is done)
	// We check the full text buffer, not just recent lines, since the PR URL
	// might scroll up as Claude continues to output text after creating the PR
	if d.matchesAny(text, d.prOpenedPatterns) {
		return StatePROpened
	}

	// Check for errors
	if d.matchesAny(recentText, d.errorPatterns) {
		return StateError
	}

	// Check for completion
	if d.matchesAny(recentText, d.completionPatterns) {
		return StateCompleted
	}

	// Check for permission prompts (highest priority waiting state)
	if d.matchesAny(recentText, d.permissionPatterns) {
		return StateWaitingPermission
	}

	// Check for questions
	if d.matchesAny(recentText, d.questionPatterns) {
		return StateWaitingQuestion
	}

	// Check for Claude Code prompt indicators (idle at input prompt)
	if d.matchesAny(recentText, d.inputWaitingPatterns) {
		return StateWaitingInput
	}

	// Default to working
	return StateWorking
}

// matchesAny checks if text matches any of the provided patterns.
func (d *Detector) matchesAny(text string, patterns []*regexp.Regexp) bool {
	for _, p := range patterns {
		if p.MatchString(text) {
			return true
		}
	}
	return false
}

// getLastNonEmptyLines returns the last n non-empty lines from a slice.
func getLastNonEmptyLines(lines []string, n int) []string {
	result := make([]string, 0, n)
	for i := len(lines) - 1; i >= 0 && len(result) < n; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			result = append([]string{line}, result...)
		}
	}
	return result
}

// StripAnsi removes ANSI escape codes from text.
// This handles both CSI sequences (ESC[...letter) and OSC sequences (ESC]...BEL).
func StripAnsi(text string) string {
	// Match ANSI escape sequences: ESC[ followed by params and a letter
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07`)
	return ansiRegex.ReplaceAllString(text, "")
}

// GetLastNonEmptyLines returns the last n non-empty lines from a slice.
// This is exported for use by other packages that need similar text processing.
func GetLastNonEmptyLines(lines []string, n int) []string {
	return getLastNonEmptyLines(lines, n)
}
