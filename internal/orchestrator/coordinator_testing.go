package orchestrator

import (
	"context"
	"sync"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// NewCoordinatorForTesting creates a minimal Coordinator for testing purposes.
// This coordinator has a working Session() method but lacks the full orchestrator
// infrastructure. It should only be used in tests where the only requirement is
// to have a coordinator that returns a specific UltraPlanSession.
//
// Note: Most methods on this coordinator will panic or behave incorrectly.
// Only Session() is guaranteed to work correctly.
func NewCoordinatorForTesting(ultraSession *UltraPlanSession) *Coordinator {
	logger := logging.NopLogger()
	manager := NewUltraPlanManager(nil, nil, ultraSession, logger)

	return &Coordinator{
		manager:      manager,
		logger:       logger,
		runningTasks: make(map[string]string),
		ctx:          context.Background(),
		cancelFunc:   func() {},
		mu:           sync.RWMutex{},
	}
}
