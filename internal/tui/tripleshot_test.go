package tui

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot"
	tuimsg "github.com/Iron-Ham/claudio/internal/tui/msg"
	"github.com/Iron-Ham/claudio/internal/tui/view"
	tea "github.com/charmbracelet/bubbletea"
)

// stubRunner is a minimal tripleshot.Runner for testing.
type stubRunner struct {
	session *tripleshot.Session
}

func (s *stubRunner) Session() *tripleshot.Session                    { return s.session }
func (s *stubRunner) SetCallbacks(_ *tripleshot.CoordinatorCallbacks) {}
func (s *stubRunner) GetWinningBranch() string                        { return "" }
func (s *stubRunner) Stop()                                           {}

func TestListenTeamwireCmd(t *testing.T) {
	t.Run("returns nil when channel is nil", func(t *testing.T) {
		m := &Model{teamwireEventCh: nil}
		cmd := m.listenTeamwireCmd()
		if cmd != nil {
			t.Error("expected nil cmd when teamwireEventCh is nil")
		}
	})

	t.Run("returns non-nil cmd when channel exists", func(t *testing.T) {
		ch := make(chan tea.Msg, 1)
		m := &Model{teamwireEventCh: ch}
		cmd := m.listenTeamwireCmd()
		if cmd == nil {
			t.Error("expected non-nil cmd when teamwireEventCh is set")
		}
	})
}

func TestHandleTeamwireCompletedResubscription(t *testing.T) {
	t.Run("re-subscribes when channel is active", func(t *testing.T) {
		ch := make(chan tea.Msg, 1)
		m := &Model{
			teamwireEventCh: ch,
			tripleShot: &TripleShotState{
				Runners: map[string]tripleshot.Runner{
					"group-1": &stubRunner{session: &tripleshot.Session{}},
					"group-2": &stubRunner{session: &tripleshot.Session{}},
				},
				UseTeamwire: true,
			},
		}

		msg := tuimsg.TeamwireCompletedMsg{
			GroupID: "group-1",
			Success: true,
			Summary: "done",
		}

		_, cmd := m.handleTeamwireCompleted(msg)
		if cmd == nil {
			t.Error("expected non-nil cmd (re-subscribe) when teamwireEventCh is active")
		}
	})

	t.Run("returns nil cmd when channel is nil", func(t *testing.T) {
		m := &Model{
			teamwireEventCh: nil,
			tripleShot: &TripleShotState{
				Runners: map[string]tripleshot.Runner{
					"group-1": &stubRunner{session: &tripleshot.Session{}},
				},
				UseTeamwire: true,
			},
		}

		msg := tuimsg.TeamwireCompletedMsg{
			GroupID: "group-1",
			Success: true,
			Summary: "done",
		}

		_, cmd := m.handleTeamwireCompleted(msg)
		if cmd != nil {
			t.Error("expected nil cmd when teamwireEventCh is nil")
		}
	})
}

func TestHandleTeamwirePhaseChangedNilGuard(t *testing.T) {
	m := &Model{teamwireEventCh: nil}

	msg := tuimsg.TeamwirePhaseChangedMsg{
		GroupID: "group-1",
		Phase:   tripleshot.PhaseWorking,
	}

	_, cmd := m.handleTeamwirePhaseChanged(msg)
	if cmd != nil {
		t.Error("expected nil cmd when teamwireEventCh is nil")
	}
}

func TestHandleTeamwireAttemptStartedNilGuard(t *testing.T) {
	m := &Model{teamwireEventCh: nil}

	msg := tuimsg.TeamwireAttemptStartedMsg{
		GroupID:      "group-1",
		AttemptIndex: 0,
		InstanceID:   "inst-1",
	}

	_, cmd := m.handleTeamwireAttemptStarted(msg)
	if cmd != nil {
		t.Error("expected nil cmd when teamwireEventCh is nil")
	}
}

func TestHandleTeamwireAttemptCompletedNilGuard(t *testing.T) {
	m := &Model{teamwireEventCh: nil}

	msg := tuimsg.TeamwireAttemptCompletedMsg{
		GroupID:      "group-1",
		AttemptIndex: 0,
	}

	_, cmd := m.handleTeamwireAttemptCompleted(msg)
	if cmd != nil {
		t.Error("expected nil cmd when teamwireEventCh is nil")
	}
}

func TestHandleTeamwireAttemptFailedNilGuard(t *testing.T) {
	m := &Model{teamwireEventCh: nil}

	msg := tuimsg.TeamwireAttemptFailedMsg{
		GroupID:      "group-1",
		AttemptIndex: 0,
		Reason:       "timeout",
	}

	_, cmd := m.handleTeamwireAttemptFailed(msg)
	if cmd != nil {
		t.Error("expected nil cmd when teamwireEventCh is nil")
	}
}

func TestHandleTeamwireJudgeStartedNilGuard(t *testing.T) {
	m := &Model{teamwireEventCh: nil}

	msg := tuimsg.TeamwireJudgeStartedMsg{
		GroupID:    "group-1",
		InstanceID: "judge-1",
	}

	_, cmd := m.handleTeamwireJudgeStarted(msg)
	if cmd != nil {
		t.Error("expected nil cmd when teamwireEventCh is nil")
	}
}

func TestHandleTeamwireJudgeStartedAutoCollapse(t *testing.T) {
	ch := make(chan tea.Msg, 1)
	m := &Model{
		teamwireEventCh: ch,
		tripleShot: &TripleShotState{
			Runners: map[string]tripleshot.Runner{
				"group-1": &stubRunner{
					session: &tripleshot.Session{
						ImplementersGroupID: "impl-group-1",
					},
				},
			},
			UseTeamwire: true,
		},
	}

	msg := tuimsg.TeamwireJudgeStartedMsg{
		GroupID:    "group-1",
		InstanceID: "judge-1",
	}

	_, cmd := m.handleTeamwireJudgeStarted(msg)
	if cmd == nil {
		t.Error("expected non-nil cmd when channel is active")
	}

	// Verify implementers group was auto-collapsed
	if m.groupViewState == nil {
		t.Fatal("expected groupViewState to be initialized")
	}
	if !m.groupViewState.IsCollapsed("impl-group-1") {
		t.Error("expected implementers group to be collapsed")
	}
}

func TestChannelReuseAcrossMultipleCoordinators(t *testing.T) {
	// Simulate the shared channel pattern: two coordinators write to the
	// same channel, and a single listener reads events from both.
	ch := make(chan tea.Msg, 16)

	// Coordinator 1's callback writes
	group1 := "group-1"
	go func() {
		ch <- tuimsg.TeamwireAttemptStartedMsg{GroupID: group1, AttemptIndex: 0, InstanceID: "inst-1"}
		ch <- tuimsg.TeamwireAttemptStartedMsg{GroupID: group1, AttemptIndex: 1, InstanceID: "inst-2"}
	}()

	// Coordinator 2's callback writes
	group2 := "group-2"
	go func() {
		ch <- tuimsg.TeamwireAttemptStartedMsg{GroupID: group2, AttemptIndex: 0, InstanceID: "inst-3"}
		ch <- tuimsg.TeamwireAttemptStartedMsg{GroupID: group2, AttemptIndex: 1, InstanceID: "inst-4"}
	}()

	// Read all 4 events and verify GroupIDs are correctly demultiplexed
	group1Count := 0
	group2Count := 0
	for i := 0; i < 4; i++ {
		msg := <-ch
		startMsg, ok := msg.(tuimsg.TeamwireAttemptStartedMsg)
		if !ok {
			t.Fatalf("expected TeamwireAttemptStartedMsg, got %T", msg)
		}
		switch startMsg.GroupID {
		case group1:
			group1Count++
		case group2:
			group2Count++
		default:
			t.Errorf("unexpected GroupID: %s", startMsg.GroupID)
		}
	}

	if group1Count != 2 {
		t.Errorf("expected 2 events from group-1, got %d", group1Count)
	}
	if group2Count != 2 {
		t.Errorf("expected 2 events from group-2, got %d", group2Count)
	}
}

func TestNeedsNewListenerLogic(t *testing.T) {
	t.Run("first coordinator creates channel and needs listener", func(t *testing.T) {
		var m Model

		// Simulate first coordinator: channel is nil
		eventCh := m.teamwireEventCh
		needsNewListener := eventCh == nil
		if !needsNewListener {
			t.Error("expected needsNewListener=true when channel is nil")
		}
		if eventCh != nil {
			t.Error("expected eventCh to be nil initially")
		}
	})

	t.Run("second coordinator reuses channel and skips listener", func(t *testing.T) {
		ch := make(chan tea.Msg, 16)
		m := Model{teamwireEventCh: ch}

		// Simulate second coordinator: channel already exists
		eventCh := m.teamwireEventCh
		needsNewListener := eventCh == nil
		if needsNewListener {
			t.Error("expected needsNewListener=false when channel already exists")
		}
		if eventCh != ch {
			t.Error("expected eventCh to be the existing channel")
		}
	})
}

func TestCleanupTripleShotNilChannel(t *testing.T) {
	t.Run("no panic when channel is already nil", func(t *testing.T) {
		m := &Model{
			teamwireEventCh: nil,
			tripleShot: &TripleShotState{
				Runners: map[string]tripleshot.Runner{},
			},
		}
		// Should not panic
		m.cleanupTripleShot()

		if m.tripleShot != nil {
			t.Error("expected tripleShot to be nil after cleanup")
		}
	})

	t.Run("closes channel exactly once", func(t *testing.T) {
		ch := make(chan tea.Msg, 1)
		m := &Model{
			teamwireEventCh: ch,
			tripleShot: &TripleShotState{
				Runners: map[string]tripleshot.Runner{},
			},
		}
		m.cleanupTripleShot()

		if m.teamwireEventCh != nil {
			t.Error("expected teamwireEventCh to be nil after cleanup")
		}
		if m.tripleShot != nil {
			t.Error("expected tripleShot to be nil after cleanup")
		}

		// Verify channel is closed by reading from it
		_, ok := <-ch
		if ok {
			t.Error("expected channel to be closed")
		}
	})
}

func TestHandleTeamwireCompletedSetsNotification(t *testing.T) {
	ch := make(chan tea.Msg, 1)
	ts := &TripleShotState{
		Runners: map[string]tripleshot.Runner{
			"group-1": &stubRunner{session: &tripleshot.Session{}},
		},
		UseTeamwire: true,
	}
	m := &Model{
		teamwireEventCh: ch,
		tripleShot:      ts,
	}

	msg := tuimsg.TeamwireCompletedMsg{
		GroupID: "group-1",
		Success: true,
		Summary: "task done",
	}

	_, _ = m.handleTeamwireCompleted(msg)

	if !ts.NeedsNotification {
		t.Error("expected NeedsNotification to be true after completion")
	}
}

// Verify that the Model's groupViewState is properly typed.
// This test exists to ensure the view.GroupViewState integration works.
var _ = (*view.GroupViewState)(nil)
