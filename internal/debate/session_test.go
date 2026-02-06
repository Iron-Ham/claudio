package debate

import (
	"strings"
	"sync"
	"testing"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/mailbox"
)

func newTestSession(t *testing.T) (*Session, *event.Bus) {
	t.Helper()
	mb := mailbox.NewMailbox(t.TempDir())
	bus := event.NewBus()
	sess := NewSession(mb, bus, "inst-a", "inst-b", "REST vs gRPC")
	return sess, bus
}

func TestNewSession(t *testing.T) {
	mb := mailbox.NewMailbox(t.TempDir())
	bus := event.NewBus()

	var received event.Event
	bus.Subscribe("debate.started", func(e event.Event) {
		received = e
	})

	sess := NewSession(mb, bus, "inst-a", "inst-b", "Which database?")

	if sess.Status() != StatusPending {
		t.Errorf("Status() = %q, want %q", sess.Status(), StatusPending)
	}
	if sess.Topic() != "Which database?" {
		t.Errorf("Topic() = %q, want %q", sess.Topic(), "Which database?")
	}
	if sess.ID() == "" {
		t.Error("ID() should not be empty")
	}
	if sess.Rounds() != 0 {
		t.Errorf("Rounds() = %d, want 0", sess.Rounds())
	}
	if len(sess.Messages()) != 0 {
		t.Errorf("Messages() length = %d, want 0", len(sess.Messages()))
	}

	// Verify event was published.
	if received == nil {
		t.Fatal("expected DebateStartedEvent to be published")
	}
	started, ok := received.(event.DebateStartedEvent)
	if !ok {
		t.Fatalf("expected DebateStartedEvent, got %T", received)
	}
	if started.DebateID != sess.ID() {
		t.Errorf("event DebateID = %q, want %q", started.DebateID, sess.ID())
	}
	if started.InstanceA != "inst-a" {
		t.Errorf("event InstanceA = %q, want %q", started.InstanceA, "inst-a")
	}
	if started.InstanceB != "inst-b" {
		t.Errorf("event InstanceB = %q, want %q", started.InstanceB, "inst-b")
	}
	if started.Topic != "Which database?" {
		t.Errorf("event Topic = %q, want %q", started.Topic, "Which database?")
	}
}

func TestNewSession_NilBus(t *testing.T) {
	mb := mailbox.NewMailbox(t.TempDir())
	sess := NewSession(mb, nil, "inst-a", "inst-b", "topic")
	if sess.Status() != StatusPending {
		t.Errorf("Status() = %q, want %q", sess.Status(), StatusPending)
	}
}

func TestChallenge(t *testing.T) {
	sess, _ := newTestSession(t)

	err := sess.Challenge("inst-a", "REST is simpler", map[string]any{"confidence": 0.8})
	if err != nil {
		t.Fatalf("Challenge() error = %v", err)
	}

	if sess.Status() != StatusActive {
		t.Errorf("Status() = %q, want %q", sess.Status(), StatusActive)
	}
	if len(sess.Messages()) != 1 {
		t.Fatalf("Messages() length = %d, want 1", len(sess.Messages()))
	}

	msg := sess.Messages()[0]
	if msg.From != "inst-a" {
		t.Errorf("msg.From = %q, want %q", msg.From, "inst-a")
	}
	if msg.To != "inst-b" {
		t.Errorf("msg.To = %q, want %q", msg.To, "inst-b")
	}
	if msg.Type != mailbox.MessageChallenge {
		t.Errorf("msg.Type = %q, want %q", msg.Type, mailbox.MessageChallenge)
	}
	if msg.Body != "REST is simpler" {
		t.Errorf("msg.Body = %q, want %q", msg.Body, "REST is simpler")
	}
	if msg.Metadata["confidence"] != 0.8 {
		t.Errorf("msg.Metadata[confidence] = %v, want 0.8", msg.Metadata["confidence"])
	}
	if msg.Metadata["debate_id"] != sess.ID() {
		t.Errorf("msg.Metadata[debate_id] = %v, want %q", msg.Metadata["debate_id"], sess.ID())
	}
	if msg.Metadata["round"] != 1 {
		t.Errorf("msg.Metadata[round] = %v, want 1", msg.Metadata["round"])
	}
}

func TestChallenge_NilMetadata(t *testing.T) {
	sess, _ := newTestSession(t)

	err := sess.Challenge("inst-a", "test", nil)
	if err != nil {
		t.Fatalf("Challenge() error = %v", err)
	}

	msg := sess.Messages()[0]
	if msg.Metadata["debate_id"] != sess.ID() {
		t.Errorf("expected debate_id in metadata even with nil input")
	}
}

func TestChallenge_InvalidParticipant(t *testing.T) {
	sess, _ := newTestSession(t)

	err := sess.Challenge("inst-c", "invalid", nil)
	if err == nil {
		t.Fatal("expected error for non-participant")
	}
	if !strings.Contains(err.Error(), "not a participant") {
		t.Errorf("error = %q, want to contain 'not a participant'", err.Error())
	}
}

func TestChallenge_AfterResolved(t *testing.T) {
	sess, _ := newTestSession(t)

	_ = sess.Challenge("inst-a", "challenge", nil)
	_ = sess.Defend("inst-b", "defense", nil)
	_ = sess.Resolve("inst-a", "consensus")

	err := sess.Challenge("inst-a", "another challenge", nil)
	if err == nil {
		t.Fatal("expected error when challenging resolved session")
	}
	if !strings.Contains(err.Error(), "already resolved") {
		t.Errorf("error = %q, want to contain 'already resolved'", err.Error())
	}
}

func TestDefend(t *testing.T) {
	sess, _ := newTestSession(t)

	_ = sess.Challenge("inst-a", "REST is simpler", nil)

	err := sess.Defend("inst-b", "gRPC has type safety", map[string]any{"confidence": 0.7})
	if err != nil {
		t.Fatalf("Defend() error = %v", err)
	}

	if sess.Rounds() != 1 {
		t.Errorf("Rounds() = %d, want 1", sess.Rounds())
	}

	msgs := sess.Messages()
	if len(msgs) != 2 {
		t.Fatalf("Messages() length = %d, want 2", len(msgs))
	}

	msg := msgs[1]
	if msg.From != "inst-b" {
		t.Errorf("msg.From = %q, want %q", msg.From, "inst-b")
	}
	if msg.To != "inst-a" {
		t.Errorf("msg.To = %q, want %q", msg.To, "inst-a")
	}
	if msg.Type != mailbox.MessageDefense {
		t.Errorf("msg.Type = %q, want %q", msg.Type, mailbox.MessageDefense)
	}
	if msg.Metadata["confidence"] != 0.7 {
		t.Errorf("msg.Metadata[confidence] = %v, want 0.7", msg.Metadata["confidence"])
	}
}

func TestDefend_NilMetadata(t *testing.T) {
	sess, _ := newTestSession(t)
	_ = sess.Challenge("inst-a", "challenge", nil)

	err := sess.Defend("inst-b", "defense", nil)
	if err != nil {
		t.Fatalf("Defend() error = %v", err)
	}

	msg := sess.Messages()[1]
	if msg.Metadata["debate_id"] != sess.ID() {
		t.Errorf("expected debate_id in metadata even with nil input")
	}
}

func TestDefend_BeforeChallenge(t *testing.T) {
	sess, _ := newTestSession(t)

	err := sess.Defend("inst-b", "defense", nil)
	if err == nil {
		t.Fatal("expected error when defending before challenge")
	}
	if !strings.Contains(err.Error(), "before a challenge") {
		t.Errorf("error = %q, want to contain 'before a challenge'", err.Error())
	}
}

func TestDefend_AfterResolved(t *testing.T) {
	sess, _ := newTestSession(t)

	_ = sess.Challenge("inst-a", "challenge", nil)
	_ = sess.Defend("inst-b", "defense", nil)
	_ = sess.Resolve("inst-a", "consensus")

	err := sess.Defend("inst-b", "another defense", nil)
	if err == nil {
		t.Fatal("expected error when defending resolved session")
	}
	if !strings.Contains(err.Error(), "already resolved") {
		t.Errorf("error = %q, want to contain 'already resolved'", err.Error())
	}
}

func TestDefend_InvalidParticipant(t *testing.T) {
	sess, _ := newTestSession(t)
	_ = sess.Challenge("inst-a", "challenge", nil)

	err := sess.Defend("inst-c", "invalid", nil)
	if err == nil {
		t.Fatal("expected error for non-participant")
	}
	if !strings.Contains(err.Error(), "not a participant") {
		t.Errorf("error = %q, want to contain 'not a participant'", err.Error())
	}
}

func TestResolve(t *testing.T) {
	sess, bus := newTestSession(t)

	var resolved event.Event
	bus.Subscribe("debate.resolved", func(e event.Event) {
		resolved = e
	})

	_ = sess.Challenge("inst-a", "REST", nil)
	_ = sess.Defend("inst-b", "gRPC", nil)

	err := sess.Resolve("inst-a", "gRPC for internal, REST for public")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if sess.Status() != StatusResolved {
		t.Errorf("Status() = %q, want %q", sess.Status(), StatusResolved)
	}

	msgs := sess.Messages()
	if len(msgs) != 3 {
		t.Fatalf("Messages() length = %d, want 3", len(msgs))
	}

	consensus := msgs[2]
	if consensus.Type != mailbox.MessageConsensus {
		t.Errorf("msg.Type = %q, want %q", consensus.Type, mailbox.MessageConsensus)
	}
	if consensus.Body != "gRPC for internal, REST for public" {
		t.Errorf("msg.Body = %q, want %q", consensus.Body, "gRPC for internal, REST for public")
	}

	// Verify event.
	if resolved == nil {
		t.Fatal("expected DebateResolvedEvent to be published")
	}
	ev, ok := resolved.(event.DebateResolvedEvent)
	if !ok {
		t.Fatalf("expected DebateResolvedEvent, got %T", resolved)
	}
	if ev.DebateID != sess.ID() {
		t.Errorf("event DebateID = %q, want %q", ev.DebateID, sess.ID())
	}
	if ev.Resolution != "gRPC for internal, REST for public" {
		t.Errorf("event Resolution = %q, want %q", ev.Resolution, "gRPC for internal, REST for public")
	}
	if ev.Rounds != 1 {
		t.Errorf("event Rounds = %d, want 1", ev.Rounds)
	}
}

func TestResolve_NilBus(t *testing.T) {
	mb := mailbox.NewMailbox(t.TempDir())
	sess := NewSession(mb, nil, "inst-a", "inst-b", "topic")

	_ = sess.Challenge("inst-a", "challenge", nil)
	_ = sess.Defend("inst-b", "defense", nil)

	err := sess.Resolve("inst-a", "consensus")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if sess.Status() != StatusResolved {
		t.Errorf("Status() = %q, want %q", sess.Status(), StatusResolved)
	}
}

func TestResolve_BeforeChallenge(t *testing.T) {
	sess, _ := newTestSession(t)

	err := sess.Resolve("inst-a", "consensus")
	if err == nil {
		t.Fatal("expected error when resolving before challenge")
	}
	if !strings.Contains(err.Error(), "before a challenge") {
		t.Errorf("error = %q, want to contain 'before a challenge'", err.Error())
	}
}

func TestResolve_AlreadyResolved(t *testing.T) {
	sess, _ := newTestSession(t)

	_ = sess.Challenge("inst-a", "challenge", nil)
	_ = sess.Defend("inst-b", "defense", nil)
	_ = sess.Resolve("inst-a", "consensus")

	err := sess.Resolve("inst-b", "another consensus")
	if err == nil {
		t.Fatal("expected error when resolving already resolved session")
	}
	if !strings.Contains(err.Error(), "already resolved") {
		t.Errorf("error = %q, want to contain 'already resolved'", err.Error())
	}
}

func TestResolve_InvalidParticipant(t *testing.T) {
	sess, _ := newTestSession(t)
	_ = sess.Challenge("inst-a", "challenge", nil)

	err := sess.Resolve("inst-c", "invalid consensus")
	if err == nil {
		t.Fatal("expected error for non-participant")
	}
	if !strings.Contains(err.Error(), "not a participant") {
		t.Errorf("error = %q, want to contain 'not a participant'", err.Error())
	}
}

func TestMultipleRounds(t *testing.T) {
	sess, _ := newTestSession(t)

	_ = sess.Challenge("inst-a", "round 1 challenge", nil)
	_ = sess.Defend("inst-b", "round 1 defense", nil)
	_ = sess.Challenge("inst-b", "round 2 challenge", nil)
	_ = sess.Defend("inst-a", "round 2 defense", nil)

	if sess.Rounds() != 2 {
		t.Errorf("Rounds() = %d, want 2", sess.Rounds())
	}
	if len(sess.Messages()) != 4 {
		t.Errorf("Messages() length = %d, want 4", len(sess.Messages()))
	}
}

func TestMessages_ReturnsCopy(t *testing.T) {
	sess, _ := newTestSession(t)
	_ = sess.Challenge("inst-a", "challenge", nil)

	msgs := sess.Messages()
	msgs[0].Body = "modified"

	// Original should not be affected.
	original := sess.Messages()
	if original[0].Body == "modified" {
		t.Error("Messages() should return a copy, not a reference to internal state")
	}
}

func TestSession_ConcurrentAccess(t *testing.T) {
	sess, _ := newTestSession(t)

	var wg sync.WaitGroup
	errs := make(chan error, 20)

	// Multiple goroutines sending challenges and defenses concurrently.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := sess.Challenge("inst-a", "concurrent challenge", nil); err != nil {
				// Resolved errors are expected; non-participant errors are not.
				if !strings.Contains(err.Error(), "already resolved") {
					errs <- err
				}
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := sess.Defend("inst-b", "concurrent defense", nil); err != nil {
				// Pending or resolved errors are expected.
				if !strings.Contains(err.Error(), "before a challenge") &&
					!strings.Contains(err.Error(), "already resolved") {
					errs <- err
				}
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("unexpected concurrent error: %v", err)
	}
}

func TestGenerateDebateID(t *testing.T) {
	id := generateDebateID("inst-a", "inst-b")
	if id != "debate-inst-a-inst-b" {
		t.Errorf("generateDebateID() = %q, want %q", id, "debate-inst-a-inst-b")
	}
}

func TestOpponent(t *testing.T) {
	sess, _ := newTestSession(t)

	tests := []struct {
		name    string
		from    string
		want    string
		wantErr bool
	}{
		{"a to b", "inst-a", "inst-b", false},
		{"b to a", "inst-b", "inst-a", false},
		{"invalid", "inst-c", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sess.opponent(tt.from)
			if (err != nil) != tt.wantErr {
				t.Errorf("opponent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("opponent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSessionStatus_Values(t *testing.T) {
	if StatusPending != "pending" {
		t.Errorf("StatusPending = %q, want %q", StatusPending, "pending")
	}
	if StatusActive != "active" {
		t.Errorf("StatusActive = %q, want %q", StatusActive, "active")
	}
	if StatusResolved != "resolved" {
		t.Errorf("StatusResolved = %q, want %q", StatusResolved, "resolved")
	}
}

func TestChallenge_DirectionBothWays(t *testing.T) {
	sess, _ := newTestSession(t)

	// Instance B can also initiate a challenge.
	err := sess.Challenge("inst-b", "I challenge from B", nil)
	if err != nil {
		t.Fatalf("Challenge from inst-b error = %v", err)
	}

	msg := sess.Messages()[0]
	if msg.From != "inst-b" {
		t.Errorf("msg.From = %q, want %q", msg.From, "inst-b")
	}
	if msg.To != "inst-a" {
		t.Errorf("msg.To = %q, want %q", msg.To, "inst-a")
	}
}

func TestChallenge_SendError(t *testing.T) {
	// Use /dev/null as session dir to force mailbox write failures.
	mb := mailbox.NewMailbox("/dev/null")
	sess := NewSession(mb, nil, "inst-a", "inst-b", "topic")

	err := sess.Challenge("inst-a", "challenge", nil)
	if err == nil {
		t.Fatal("expected error from mailbox send failure")
	}
	if !strings.Contains(err.Error(), "send challenge") {
		t.Errorf("error = %q, want to contain 'send challenge'", err.Error())
	}
}

func TestDefend_SendError(t *testing.T) {
	mb := mailbox.NewMailbox(t.TempDir())
	sess := NewSession(mb, nil, "inst-a", "inst-b", "topic")

	// First challenge succeeds with a real dir.
	_ = sess.Challenge("inst-a", "challenge", nil)

	// Replace mailbox with one that uses an invalid dir.
	sess.mb = mailbox.NewMailbox("/dev/null")

	err := sess.Defend("inst-b", "defense", nil)
	if err == nil {
		t.Fatal("expected error from mailbox send failure")
	}
	if !strings.Contains(err.Error(), "send defense") {
		t.Errorf("error = %q, want to contain 'send defense'", err.Error())
	}
}

func TestResolve_SendError(t *testing.T) {
	mb := mailbox.NewMailbox(t.TempDir())
	sess := NewSession(mb, nil, "inst-a", "inst-b", "topic")

	_ = sess.Challenge("inst-a", "challenge", nil)

	// Replace mailbox with one that uses an invalid dir.
	sess.mb = mailbox.NewMailbox("/dev/null")

	err := sess.Resolve("inst-b", "consensus")
	if err == nil {
		t.Fatal("expected error from mailbox send failure")
	}
	if !strings.Contains(err.Error(), "send consensus") {
		t.Errorf("error = %q, want to contain 'send consensus'", err.Error())
	}
}

func TestResolve_AfterChallengeOnly(t *testing.T) {
	// A debate can be resolved even without a defense round.
	sess, _ := newTestSession(t)

	_ = sess.Challenge("inst-a", "challenge", nil)

	err := sess.Resolve("inst-b", "I agree with the challenge")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if sess.Status() != StatusResolved {
		t.Errorf("Status() = %q, want %q", sess.Status(), StatusResolved)
	}
	if sess.Rounds() != 0 {
		t.Errorf("Rounds() = %d, want 0 (no defense was issued)", sess.Rounds())
	}
}
