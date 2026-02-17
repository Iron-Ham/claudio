package pipeline

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Iron-Ham/claudio/internal/debate"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/mailbox"
	"github.com/Iron-Ham/claudio/internal/taskqueue"
)

// ConflictPair identifies two tasks that touched overlapping files.
type ConflictPair struct {
	TaskA        string   // ID of the first task
	TaskB        string   // ID of the second task
	OverlapFiles []string // Files touched by both tasks
}

// DebateResolution holds the outcome of a single debate session.
type DebateResolution struct {
	SessionID  string
	TaskA      string
	TaskB      string
	Resolution string
	Files      []string
}

// DebateCoordinator identifies file conflicts between completed tasks and
// runs structured debate sessions to reconcile them.
type DebateCoordinator struct {
	mb          *mailbox.Mailbox
	bus         *event.Bus
	resolutions []DebateResolution
}

// NewDebateCoordinator creates a DebateCoordinator backed by the given
// mailbox and event bus.
func NewDebateCoordinator(mb *mailbox.Mailbox, bus *event.Bus) *DebateCoordinator {
	return &DebateCoordinator{
		mb:  mb,
		bus: bus,
	}
}

// FindConflicts identifies pairs of completed tasks that touched overlapping files.
func (dc *DebateCoordinator) FindConflicts(completedTasks []taskqueue.QueuedTask) []ConflictPair {
	// Build file → task IDs index (only completed tasks).
	fileToTasks := make(map[string][]string)
	for _, t := range completedTasks {
		if t.Status != taskqueue.TaskCompleted {
			continue
		}
		for _, f := range t.Files {
			fileToTasks[f] = append(fileToTasks[f], t.ID)
		}
	}

	// Collect unique pairs with overlapping files.
	type pairKey struct{ a, b string }
	pairFiles := make(map[pairKey][]string)

	for file, taskIDs := range fileToTasks {
		if len(taskIDs) < 2 {
			continue
		}
		for i := 0; i < len(taskIDs); i++ {
			for j := i + 1; j < len(taskIDs); j++ {
				a, b := taskIDs[i], taskIDs[j]
				// Normalize pair order for deduplication.
				if a > b {
					a, b = b, a
				}
				key := pairKey{a, b}
				pairFiles[key] = append(pairFiles[key], file)
			}
		}
	}

	// Build sorted result.
	pairs := make([]ConflictPair, 0, len(pairFiles))
	for key, files := range pairFiles {
		sort.Strings(files)
		pairs = append(pairs, ConflictPair{
			TaskA:        key.a,
			TaskB:        key.b,
			OverlapFiles: files,
		})
	}
	// Sort pairs deterministically.
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].TaskA != pairs[j].TaskA {
			return pairs[i].TaskA < pairs[j].TaskA
		}
		return pairs[i].TaskB < pairs[j].TaskB
	})
	return pairs
}

// RunDebates creates a debate session for each conflict pair, records
// positions, and synthesizes resolutions. The debate is a structured record
// (non-interactive): positions are captured from task metadata rather than
// requiring live multi-round exchanges.
func (dc *DebateCoordinator) RunDebates(_ context.Context, conflicts []ConflictPair, tasks []taskqueue.QueuedTask) ([]DebateResolution, error) {
	// Build task index for lookup.
	taskIndex := make(map[string]taskqueue.QueuedTask, len(tasks))
	for _, t := range tasks {
		taskIndex[t.ID] = t
	}

	var resolutions []DebateResolution
	for _, conflict := range conflicts {
		taskA := taskIndex[conflict.TaskA]
		taskB := taskIndex[conflict.TaskB]

		topic := fmt.Sprintf("File conflict resolution: %s", strings.Join(conflict.OverlapFiles, ", "))

		session := debate.NewSession(dc.mb, dc.bus, conflict.TaskA, conflict.TaskB, topic)

		// Record positions from task descriptions.
		challengeBody := fmt.Sprintf("%s approach: %s", taskA.Title, taskA.Description)
		if err := session.Challenge(conflict.TaskA, challengeBody, nil); err != nil {
			return resolutions, fmt.Errorf("debate challenge for %s: %w", conflict.TaskA, err)
		}

		defendBody := fmt.Sprintf("%s approach: %s", taskB.Title, taskB.Description)
		if err := session.Defend(conflict.TaskB, defendBody, nil); err != nil {
			return resolutions, fmt.Errorf("debate defense for %s: %w", conflict.TaskB, err)
		}

		// Synthesize resolution.
		resolution := fmt.Sprintf("Both tasks completed. Potential merge conflicts in: %s",
			strings.Join(conflict.OverlapFiles, ", "))
		if err := session.Resolve(conflict.TaskA, resolution); err != nil {
			return resolutions, fmt.Errorf("debate resolve for %s vs %s: %w",
				conflict.TaskA, conflict.TaskB, err)
		}

		resolutions = append(resolutions, DebateResolution{
			SessionID:  session.ID(),
			TaskA:      conflict.TaskA,
			TaskB:      conflict.TaskB,
			Resolution: resolution,
			Files:      conflict.OverlapFiles,
		})
	}

	dc.resolutions = resolutions
	return resolutions, nil
}

// Resolutions returns a copy of all resolved debates.
func (dc *DebateCoordinator) Resolutions() []DebateResolution {
	out := make([]DebateResolution, len(dc.resolutions))
	copy(out, dc.resolutions)
	return out
}

// formatDebateContext formats debate resolutions into text suitable for
// injection into the review team's lead prompt.
func formatDebateContext(resolutions []DebateResolution) string {
	if len(resolutions) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n## File Conflict Debates\n\n")
	sb.WriteString("The following file conflicts were detected between execution tasks ")
	sb.WriteString("and debated for reconciliation:\n\n")

	for _, r := range resolutions {
		sb.WriteString(fmt.Sprintf("- **%s vs %s** (files: %s): %s\n",
			r.TaskA, r.TaskB,
			strings.Join(r.Files, ", "),
			r.Resolution))
	}

	return sb.String()
}
