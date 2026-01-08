package instance

import (
	"regexp"
	"strings"
)

// WaitingState represents different types of waiting conditions Claude can be in
type WaitingState int

const (
	// StateWorking means Claude is actively working (not waiting)
	StateWorking WaitingState = iota
	// StateWaitingPermission means Claude is asking for permission to perform an action
	StateWaitingPermission
	// StateWaitingQuestion means Claude is asking the user a question
	StateWaitingQuestion
	// StateWaitingInput means Claude is waiting for general input
	StateWaitingInput
	// StateCompleted means Claude has finished its task
	StateCompleted
	// StateError means Claude encountered an error
	StateError
	// StatePROpened means Claude opened a pull request (PR URL detected in output)
	StatePROpened
)

// String returns a human-readable string for the waiting state
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
func (s WaitingState) IsWaiting() bool {
	return s == StateWaitingPermission || s == StateWaitingQuestion || s == StateWaitingInput
}

// Detector analyzes Claude's output to determine if it's waiting for user input
type Detector struct {
	// Compiled regex patterns for efficiency
	permissionPatterns []*regexp.Regexp
	questionPatterns   []*regexp.Regexp
	completionPatterns []*regexp.Regexp
	errorPatterns      []*regexp.Regexp
	workingPatterns    []*regexp.Regexp
	prOpenedPatterns   []*regexp.Regexp
}

// NewDetector creates a new output state detector
func NewDetector() *Detector {
	d := &Detector{}

	// Permission prompts - Claude asking for approval to do something
	// These typically appear with Yes/No options or require explicit approval
	permissionStrings := []string{
		// Standard permission prompts
		`(?i)do you want (?:me )?to (?:proceed|continue|run|execute|apply|make)`,
		`(?i)(?:shall|should|can|may) I (?:proceed|continue|go ahead|run|execute|apply)`,
		`(?i)(?:allow|permit|approve) (?:this|the) (?:action|change|operation)`,
		`(?i)\[Y(?:es)?/[Nn](?:o)?\]`,                                     // [Y/N] or [Yes/No] prompts
		`(?i)\(y(?:es)?/n(?:o)?\)`,                                        // (y/n) or (yes/no) prompts
		`(?i)press (?:y|enter) to (?:confirm|continue|proceed|approve)`,   // Press y to confirm
		`(?i)type ['"]?(?:yes|y)['"]? to (?:confirm|continue|proceed)`,    // Type yes to confirm
		`(?i)waiting for (?:your )?(?:approval|confirmation|permission)`,  // Explicit waiting
		`(?i)requires? (?:your )?(?:approval|confirmation|permission)`,    // Requires permission
	}

	// Question patterns - Claude asking for information or clarification
	questionStrings := []string{
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

	// Completion patterns - Claude has finished its task
	completionStrings := []string{
		`(?i)(?:task|work|implementation|changes?) (?:is |are )?(?:complete|done|finished)`,
		`(?i)(?:i'?ve|I have) (?:completed|finished|done)`,
		`(?i)(?:successfully|all done|that'?s it)`,
		`(?i)let me know if (?:you need|there'?s) anything else`,
		`(?i)is there anything else`,
	}

	// Error patterns - Claude encountered an issue
	errorStrings := []string{
		`(?i)(?:error|exception|failed|failure)(?::|!)`,
		`(?i)(?:could not|couldn'?t|unable to|cannot|can'?t) (?:complete|finish|proceed|continue)`,
		`(?i)(?:fatal|critical|severe) (?:error|issue|problem)`,
		`(?i)process (?:exited|terminated|crashed|died)`,
	}

	// Working patterns - Claude is actively doing something (these override waiting detection)
	// Note: These should be specific enough to not match completion phrases like "let me know if..."
	workingStrings := []string{
		`(?i)(?:reading|writing|editing|creating|modifying|analyzing|searching|running|executing|building|compiling|testing)\.{3}`,
		`(?i)(?:let me (?:check|look|see|analyze|examine|investigate|read|search|find)|i'?ll (?:check|look|start|begin)|going to (?:check|look|analyze|implement)|about to (?:start|begin|run))`,
		`(?i)(?:working on|processing|loading|fetching)`,
		`⠋|⠙|⠹|⠸|⠼|⠴|⠦|⠧|⠇|⠏`, // Spinner characters
	}

	// PR opened patterns - detect when a pull request URL appears in output
	// This indicates Claude has successfully created a PR via gh pr create
	prOpenedStrings := []string{
		// GitHub PR URL pattern: https://github.com/owner/repo/pull/123
		`https://github\.com/[^/]+/[^/]+/pull/\d+`,
	}

	// Compile all patterns
	d.permissionPatterns = compilePatterns(permissionStrings)
	d.questionPatterns = compilePatterns(questionStrings)
	d.completionPatterns = compilePatterns(completionStrings)
	d.errorPatterns = compilePatterns(errorStrings)
	d.workingPatterns = compilePatterns(workingStrings)
	d.prOpenedPatterns = compilePatterns(prOpenedStrings)

	return d
}

// compilePatterns compiles a list of regex pattern strings
func compilePatterns(patterns []string) []*regexp.Regexp {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}

// Detect analyzes output and returns the detected waiting state
// It examines the last portion of output (most recent content) for patterns
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
	text = stripAnsi(text)

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

	// Default to working
	return StateWorking
}

// matchesAny checks if text matches any of the provided patterns
func (d *Detector) matchesAny(text string, patterns []*regexp.Regexp) bool {
	for _, p := range patterns {
		if p.MatchString(text) {
			return true
		}
	}
	return false
}

// getLastNonEmptyLines returns the last n non-empty lines from a slice
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

// stripAnsi removes ANSI escape codes from text
func stripAnsi(text string) string {
	// Match ANSI escape sequences: ESC[ followed by params and a letter
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07`)
	return ansiRegex.ReplaceAllString(text, "")
}
