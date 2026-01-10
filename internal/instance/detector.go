package instance

import (
	"github.com/Iron-Ham/claudio/internal/instance/detect"
)

// WaitingState represents different types of waiting conditions Claude can be in.
// This is an alias to detect.WaitingState for backwards compatibility.
// New code should import from internal/instance/detect directly.
type WaitingState = detect.WaitingState

// Re-export state constants for backwards compatibility.
// New code should import from internal/instance/detect directly.
const (
	StateWorking           = detect.StateWorking
	StateWaitingPermission = detect.StateWaitingPermission
	StateWaitingQuestion   = detect.StateWaitingQuestion
	StateWaitingInput      = detect.StateWaitingInput
	StateCompleted         = detect.StateCompleted
	StateError             = detect.StateError
	StatePROpened          = detect.StatePROpened
)

// Detector analyzes Claude's output to determine if it's waiting for user input.
// This is an alias to detect.Detector for backwards compatibility.
// New code should import from internal/instance/detect directly.
type Detector = detect.Detector

// NewDetector creates a new output state detector.
// This is a wrapper for detect.NewDetector for backwards compatibility.
// New code should import from internal/instance/detect directly.
func NewDetector() *Detector {
	return detect.NewDetector()
}
