// Package ultraplan provides UI components for ultra-plan mode.
package ultraplan

import (
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"
)

// NotificationSeverity represents the urgency level of a notification.
type NotificationSeverity int

const (
	// SeverityInfo indicates an informational notification.
	SeverityInfo NotificationSeverity = iota
	// SeverityWarning indicates a notification that needs user attention.
	SeverityWarning
	// SeverityCritical indicates a notification requiring immediate action.
	SeverityCritical
)

// NotificationType identifies the category of notification.
type NotificationType string

const (
	// NotificationPlanReady indicates a plan is ready for review.
	NotificationPlanReady NotificationType = "plan_ready"
	// NotificationSynthesisReady indicates synthesis is complete and awaiting approval.
	NotificationSynthesisReady NotificationType = "synthesis_ready"
	// NotificationRevisionStarted indicates the revision phase has begun.
	NotificationRevisionStarted NotificationType = "revision_started"
	// NotificationConsolidationPaused indicates consolidation is paused due to conflicts.
	NotificationConsolidationPaused NotificationType = "consolidation_paused"
	// NotificationGroupDecision indicates a group decision is required.
	NotificationGroupDecision NotificationType = "group_decision"
	// NotificationComplete indicates the ultra-plan has completed.
	NotificationComplete NotificationType = "complete"
	// NotificationError indicates an error occurred.
	NotificationError NotificationType = "error"
)

// Notification represents a user-facing notification in ultra-plan mode.
type Notification struct {
	Type      NotificationType
	Message   string
	Severity  NotificationSeverity
	Timestamp time.Time
	Phase     orchestrator.UltraPlanPhase
	// AutoDismiss indicates whether this notification should auto-dismiss after timeout.
	AutoDismiss bool
}

// NotificationConfig holds configuration for notification behavior.
type NotificationConfig struct {
	// Enabled controls whether notifications are sent at all.
	Enabled bool
	// UseSound enables audio notifications on supported platforms.
	UseSound bool
	// SoundPath is the path to the sound file on macOS.
	SoundPath string
	// AutoDismissTimeout is how long before auto-dismissing non-critical notifications.
	AutoDismissTimeout time.Duration
}

// DefaultNotificationConfig returns default notification configuration.
func DefaultNotificationConfig() NotificationConfig {
	return NotificationConfig{
		Enabled:            true,
		UseSound:           true,
		SoundPath:          "/System/Library/Sounds/Glass.aiff",
		AutoDismissTimeout: 5 * time.Second,
	}
}

// NotificationConfigFromViper creates a NotificationConfig from viper settings.
func NotificationConfigFromViper() NotificationConfig {
	cfg := DefaultNotificationConfig()
	cfg.Enabled = viper.GetBool("ultraplan.notifications.enabled")
	cfg.UseSound = viper.GetBool("ultraplan.notifications.use_sound")
	if path := viper.GetString("ultraplan.notifications.sound_path"); path != "" {
		cfg.SoundPath = path
	}
	if timeout := viper.GetDuration("ultraplan.notifications.auto_dismiss_timeout"); timeout > 0 {
		cfg.AutoDismissTimeout = timeout
	}
	return cfg
}

// NotificationManager centralizes notification handling for ultra-plan mode.
// It manages a queue of notifications, tracks which notifications have been shown
// to prevent duplicates, and handles notification delivery (bell, sound).
type NotificationManager struct {
	mu sync.RWMutex

	// Queue of pending notifications
	queue []*Notification

	// Current notification being displayed (if any)
	current *Notification

	// Configuration
	config NotificationConfig

	// Deduplication tracking - maps phase to whether we've notified for it
	lastNotifiedPhase      orchestrator.UltraPlanPhase
	lastConsolidationPhase orchestrator.ConsolidationPhase
	notifiedGroupDecision  bool

	// Timestamp when current notification was displayed (for auto-dismiss)
	currentDisplayedAt time.Time
}

// NewNotificationManager creates a new NotificationManager with the given config.
func NewNotificationManager(config NotificationConfig) *NotificationManager {
	return &NotificationManager{
		config: config,
		queue:  make([]*Notification, 0),
	}
}

// AddNotification adds a notification to the queue.
// If there's no current notification, this becomes the current notification.
func (nm *NotificationManager) AddNotification(n Notification) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	n.Timestamp = time.Now()
	nm.queue = append(nm.queue, &n)

	// If no current notification, promote from queue immediately
	if nm.current == nil {
		nm.promoteFromQueue()
	}
}

// GetCurrentNotification returns the current notification, or nil if none.
func (nm *NotificationManager) GetCurrentNotification() *Notification {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.current
}

// DismissNotification dismisses the current notification and promotes the next
// one from the queue if available.
func (nm *NotificationManager) DismissNotification() {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	nm.current = nil
	nm.currentDisplayedAt = time.Time{}
	nm.promoteFromQueue()
}

// CheckTimeout checks if the current notification should be auto-dismissed
// based on its timeout. Returns true if the notification was dismissed.
func (nm *NotificationManager) CheckTimeout() bool {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if nm.current == nil {
		return false
	}

	if !nm.current.AutoDismiss {
		return false
	}

	if nm.currentDisplayedAt.IsZero() {
		return false
	}

	if time.Since(nm.currentDisplayedAt) >= nm.config.AutoDismissTimeout {
		nm.current = nil
		nm.currentDisplayedAt = time.Time{}
		nm.promoteFromQueue()
		return true
	}

	return false
}

// promoteFromQueue moves the next notification from the queue to current.
// Must be called with lock held.
func (nm *NotificationManager) promoteFromQueue() {
	if len(nm.queue) > 0 {
		nm.current = nm.queue[0]
		nm.queue = nm.queue[1:]
		nm.currentDisplayedAt = time.Now()
	}
}

// HasPending returns true if there are pending notifications in the queue
// or a current notification is being displayed.
func (nm *NotificationManager) HasPending() bool {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.current != nil || len(nm.queue) > 0
}

// QueueLength returns the number of pending notifications in the queue
// (not including the current notification).
func (nm *NotificationManager) QueueLength() int {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return len(nm.queue)
}

// ClearAll clears all notifications (current and queued).
func (nm *NotificationManager) ClearAll() {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	nm.current = nil
	nm.queue = nm.queue[:0]
	nm.currentDisplayedAt = time.Time{}
}

// NeedsNotification returns true if a notification was recently added and
// user should be alerted. This is consumed on read (resets to false).
func (nm *NotificationManager) NeedsNotification() bool {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// Return true if we have a current notification that was just promoted
	// (displayed at is recent, within 100ms)
	if nm.current != nil && !nm.currentDisplayedAt.IsZero() {
		if time.Since(nm.currentDisplayedAt) < 100*time.Millisecond {
			return true
		}
	}
	return false
}

// CheckForPhaseNotification checks the ultra-plan state and generates
// appropriate notifications for phase changes that need user attention.
// Returns a notification if one should be shown, nil otherwise.
func (nm *NotificationManager) CheckForPhaseNotification(session *orchestrator.UltraPlanSession) *Notification {
	if session == nil {
		return nil
	}

	nm.mu.Lock()
	defer nm.mu.Unlock()

	// Check for synthesis phase (user may want to review)
	if session.Phase == orchestrator.PhaseSynthesis && nm.lastNotifiedPhase != orchestrator.PhaseSynthesis {
		nm.lastNotifiedPhase = orchestrator.PhaseSynthesis
		return &Notification{
			Type:        NotificationSynthesisReady,
			Message:     "Synthesis complete - review results",
			Severity:    SeverityWarning,
			Phase:       orchestrator.PhaseSynthesis,
			AutoDismiss: false, // Requires user action
			Timestamp:   time.Now(),
		}
	}

	// Check for revision phase (issues were found, user may want to know)
	if session.Phase == orchestrator.PhaseRevision && nm.lastNotifiedPhase != orchestrator.PhaseRevision {
		nm.lastNotifiedPhase = orchestrator.PhaseRevision
		return &Notification{
			Type:        NotificationRevisionStarted,
			Message:     "Revision phase started - addressing issues",
			Severity:    SeverityInfo,
			Phase:       orchestrator.PhaseRevision,
			AutoDismiss: true,
			Timestamp:   time.Now(),
		}
	}

	// Check for consolidation pause (conflict detected, needs user attention)
	if session.Phase == orchestrator.PhaseConsolidating && session.Consolidation != nil {
		if session.Consolidation.Phase == orchestrator.ConsolidationPaused &&
			nm.lastConsolidationPhase != orchestrator.ConsolidationPaused {
			nm.lastConsolidationPhase = orchestrator.ConsolidationPaused
			return &Notification{
				Type:        NotificationConsolidationPaused,
				Message:     "Consolidation paused - conflicts detected",
				Severity:    SeverityCritical,
				Phase:       orchestrator.PhaseConsolidating,
				AutoDismiss: false, // Requires user action
				Timestamp:   time.Now(),
			}
		}
		// Track consolidation phase changes
		nm.lastConsolidationPhase = session.Consolidation.Phase
	}

	// Check for group decision needed (partial success/failure)
	// Only notify once when we enter the awaiting decision state
	if session.GroupDecision != nil && session.GroupDecision.AwaitingDecision {
		if !nm.notifiedGroupDecision {
			nm.notifiedGroupDecision = true
			return &Notification{
				Type:        NotificationGroupDecision,
				Message:     "Group decision required - partial completion",
				Severity:    SeverityCritical,
				Phase:       session.Phase,
				AutoDismiss: false, // Requires user action
				Timestamp:   time.Now(),
			}
		}
		return nil
	}

	// Reset group decision notification flag when no longer awaiting
	// (so we can notify again if another group decision occurs later)
	if nm.notifiedGroupDecision {
		nm.notifiedGroupDecision = false
	}

	// Check for completion
	if session.Phase == orchestrator.PhaseComplete && nm.lastNotifiedPhase != orchestrator.PhaseComplete {
		nm.lastNotifiedPhase = orchestrator.PhaseComplete
		return &Notification{
			Type:        NotificationComplete,
			Message:     "Ultra-plan completed successfully",
			Severity:    SeverityInfo,
			Phase:       orchestrator.PhaseComplete,
			AutoDismiss: true,
			Timestamp:   time.Now(),
		}
	}

	return nil
}

// NotifyPlanReady adds a notification for when a plan is ready for review.
func (nm *NotificationManager) NotifyPlanReady(taskCount, groupCount int) {
	nm.mu.Lock()
	nm.lastNotifiedPhase = orchestrator.PhaseRefresh
	nm.mu.Unlock()

	nm.AddNotification(Notification{
		Type:        NotificationPlanReady,
		Message:     "Plan ready for review",
		Severity:    SeverityWarning,
		Phase:       orchestrator.PhaseRefresh,
		AutoDismiss: false, // Requires user action
	})
}

// NotifyError adds an error notification.
func (nm *NotificationManager) NotifyError(message string) {
	nm.AddNotification(Notification{
		Type:        NotificationError,
		Message:     message,
		Severity:    SeverityCritical,
		AutoDismiss: false,
	})
}

// ResetPhaseTracking resets all phase tracking state.
// Call this when starting a new ultra-plan session.
func (nm *NotificationManager) ResetPhaseTracking() {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	nm.lastNotifiedPhase = ""
	nm.lastConsolidationPhase = ""
	nm.notifiedGroupDecision = false
}

// UpdateConfig updates the notification configuration.
func (nm *NotificationManager) UpdateConfig(config NotificationConfig) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.config = config
}

// GetConfig returns a copy of the current configuration.
func (nm *NotificationManager) GetConfig() NotificationConfig {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.config
}

// NotifyUserCmd returns a tea.Cmd that notifies the user via bell and optional sound.
// This is the bubbletea command for delivering the notification side-effect.
func NotifyUserCmd(config NotificationConfig) tea.Cmd {
	return func() tea.Msg {
		if !config.Enabled {
			return nil
		}

		// Always ring terminal bell
		_, _ = os.Stdout.Write([]byte{'\a'})

		// Optionally play system sound on macOS
		if runtime.GOOS == "darwin" && config.UseSound {
			soundPath := config.SoundPath
			if soundPath == "" {
				soundPath = "/System/Library/Sounds/Glass.aiff"
			}
			// Start in background so it doesn't block
			_ = exec.Command("afplay", soundPath).Start()
		}
		return NotificationDeliveredMsg{}
	}
}

// NotificationDeliveredMsg is sent after a notification has been delivered to the user.
type NotificationDeliveredMsg struct{}

// SeverityString returns a human-readable string for the severity level.
func (s NotificationSeverity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// TypeString returns a human-readable description of the notification type.
func (t NotificationType) Description() string {
	switch t {
	case NotificationPlanReady:
		return "Plan Ready"
	case NotificationSynthesisReady:
		return "Synthesis Complete"
	case NotificationRevisionStarted:
		return "Revision Started"
	case NotificationConsolidationPaused:
		return "Consolidation Paused"
	case NotificationGroupDecision:
		return "Decision Required"
	case NotificationComplete:
		return "Complete"
	case NotificationError:
		return "Error"
	default:
		return string(t)
	}
}
