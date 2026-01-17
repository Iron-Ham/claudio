package orchestrator

import (
	"testing"
)

// TestResolveByNumber tests the sidebar number resolution functionality.
// This allows users to reference instances by their visible sidebar number.
func TestResolveByNumber(t *testing.T) {
	// Create a minimal orchestrator for testing
	orch := &Orchestrator{}

	// Create a session with multiple instances
	session := NewSession("test", "/repo")
	inst1 := NewInstance("task 1")
	inst1.ID = "inst-1"
	inst2 := NewInstance("task 2")
	inst2.ID = "inst-2"
	inst3 := NewInstance("task 3")
	inst3.ID = "inst-3"
	session.Instances = []*Instance{inst1, inst2, inst3}

	tests := []struct {
		name        string
		ref         string
		expected    *Instance
		wasNumeric  bool
		expectError bool
	}{
		// Plain number syntax (1-indexed)
		{name: "plain number 1", ref: "1", expected: inst1, wasNumeric: true, expectError: false},
		{name: "plain number 2", ref: "2", expected: inst2, wasNumeric: true, expectError: false},
		{name: "plain number 3", ref: "3", expected: inst3, wasNumeric: true, expectError: false},

		// Explicit # prefix syntax (1-indexed)
		{name: "hash prefix #1", ref: "#1", expected: inst1, wasNumeric: true, expectError: false},
		{name: "hash prefix #2", ref: "#2", expected: inst2, wasNumeric: true, expectError: false},
		{name: "hash prefix #3", ref: "#3", expected: inst3, wasNumeric: true, expectError: false},

		// Out of range - returns error (doesn't fall through)
		{name: "number 0 (invalid)", ref: "0", expected: nil, wasNumeric: true, expectError: true},
		{name: "number 4 (out of range)", ref: "4", expected: nil, wasNumeric: true, expectError: true},
		{name: "number 100 (way out of range)", ref: "100", expected: nil, wasNumeric: true, expectError: true},
		{name: "negative number", ref: "-1", expected: nil, wasNumeric: true, expectError: true},

		// Non-numeric strings - returns nil (falls through to other resolution)
		{name: "non-numeric string", ref: "abc", expected: nil, wasNumeric: false, expectError: false},
		{name: "task name", ref: "task 1", expected: nil, wasNumeric: false, expectError: false},
		{name: "instance ID", ref: "inst-1", expected: nil, wasNumeric: false, expectError: false},

		// Hash with non-number - explicit numeric attempt, should error
		{name: "hash with non-number", ref: "#abc", expected: nil, wasNumeric: true, expectError: true},

		// Edge cases
		{name: "empty string", ref: "", expected: nil, wasNumeric: false, expectError: false},
		{name: "just hash", ref: "#", expected: nil, wasNumeric: true, expectError: true},
		{name: "leading zeros", ref: "01", expected: inst1, wasNumeric: true, expectError: false},
		{name: "hash with leading zeros", ref: "#01", expected: inst1, wasNumeric: true, expectError: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, wasNumeric, err := orch.resolveByNumber(session, tt.ref)
			if tt.expectError {
				if err == nil {
					t.Errorf("resolveByNumber(%q) expected error, got nil", tt.ref)
				}
				return
			}
			if err != nil {
				t.Errorf("resolveByNumber(%q) unexpected error: %v", tt.ref, err)
				return
			}
			if wasNumeric != tt.wasNumeric {
				t.Errorf("resolveByNumber(%q) wasNumeric = %v, want %v", tt.ref, wasNumeric, tt.wasNumeric)
			}
			if result != tt.expected {
				var resultID, expectedID string
				if result != nil {
					resultID = result.ID
				}
				if tt.expected != nil {
					expectedID = tt.expected.ID
				}
				t.Errorf("resolveByNumber(%q) = %v (ID=%s), want %v (ID=%s)",
					tt.ref, result, resultID, tt.expected, expectedID)
			}
		})
	}
}

// TestResolveByNumber_EmptySession tests sidebar number resolution with an empty session.
func TestResolveByNumber_EmptySession(t *testing.T) {
	orch := &Orchestrator{}
	session := NewSession("test", "/repo")
	// session.Instances is empty

	tests := []string{"1", "#1", "0", "#0"}
	for _, ref := range tests {
		t.Run(ref, func(t *testing.T) {
			result, _, err := orch.resolveByNumber(session, ref)
			// All numeric references should return error for empty session
			if err == nil {
				t.Errorf("resolveByNumber(%q) with empty session expected error, got nil", ref)
			}
			if result != nil {
				t.Errorf("resolveByNumber(%q) with empty session = %v, want nil", ref, result)
			}
		})
	}
}

// TestResolveInstanceReference tests the full instance reference resolution,
// including sidebar numbers, instance IDs, and task name substrings.
func TestResolveInstanceReference(t *testing.T) {
	orch := &Orchestrator{}

	// Create a session with instances
	session := NewSession("test", "/repo")
	inst1 := NewInstance("Write tests for authentication")
	inst1.ID = "abc12345"
	inst2 := NewInstance("Implement new feature")
	inst2.ID = "def67890"
	inst3 := NewInstance("Fix bug in login")
	inst3.ID = "ghi11111"
	session.Instances = []*Instance{inst1, inst2, inst3}

	tests := []struct {
		name        string
		ref         string
		expected    *Instance
		expectError bool
	}{
		// Sidebar number resolution (highest priority)
		{name: "sidebar number 1", ref: "1", expected: inst1, expectError: false},
		{name: "sidebar number #2", ref: "#2", expected: inst2, expectError: false},
		{name: "sidebar number 3", ref: "3", expected: inst3, expectError: false},

		// Exact ID match
		{name: "exact ID match", ref: "abc12345", expected: inst1, expectError: false},
		{name: "exact ID match 2", ref: "def67890", expected: inst2, expectError: false},

		// Task name substring match
		{name: "task substring - tests", ref: "tests", expected: inst1, expectError: false},
		{name: "task substring - feature", ref: "feature", expected: inst2, expectError: false},
		{name: "task substring - login", ref: "login", expected: inst3, expectError: false},
		{name: "task substring - case insensitive", ref: "LOGIN", expected: inst3, expectError: false},

		// No match
		{name: "no match", ref: "nonexistent", expected: nil, expectError: true},

		// Out of range sidebar number falls through to other resolution
		{name: "out of range number 4", ref: "4", expected: nil, expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := orch.ResolveInstanceReference(session, tt.ref)
			if tt.expectError {
				if err == nil {
					t.Errorf("ResolveInstanceReference(%q) expected error, got nil", tt.ref)
				}
				return
			}
			if err != nil {
				t.Errorf("ResolveInstanceReference(%q) unexpected error: %v", tt.ref, err)
				return
			}
			if result != tt.expected {
				var resultID, expectedID string
				if result != nil {
					resultID = result.ID
				}
				if tt.expected != nil {
					expectedID = tt.expected.ID
				}
				t.Errorf("ResolveInstanceReference(%q) = %v (ID=%s), want %v (ID=%s)",
					tt.ref, result, resultID, tt.expected, expectedID)
			}
		})
	}
}

// TestResolveInstanceReference_AmbiguousMatch tests that ambiguous task name matches
// return an error.
func TestResolveInstanceReference_AmbiguousMatch(t *testing.T) {
	orch := &Orchestrator{}

	session := NewSession("test", "/repo")
	inst1 := NewInstance("Write unit tests")
	inst1.ID = "inst-1"
	inst2 := NewInstance("Write integration tests")
	inst2.ID = "inst-2"
	session.Instances = []*Instance{inst1, inst2}

	// "tests" matches both instances - should return an error
	_, err := orch.ResolveInstanceReference(session, "tests")
	if err == nil {
		t.Error("ResolveInstanceReference('tests') should return error for ambiguous match")
	}

	// But sidebar numbers should still work unambiguously
	result, err := orch.ResolveInstanceReference(session, "1")
	if err != nil {
		t.Errorf("ResolveInstanceReference('1') unexpected error: %v", err)
	}
	if result != inst1 {
		t.Errorf("ResolveInstanceReference('1') = %v, want %v", result, inst1)
	}
}

// TestResolveInstanceReference_NumberVsTaskName tests that numeric strings
// are resolved as sidebar numbers first, and out-of-range numbers return
// clear errors rather than falling back to task name matching.
func TestResolveInstanceReference_NumberVsTaskName(t *testing.T) {
	orch := &Orchestrator{}

	session := NewSession("test", "/repo")
	// Create instances where one has a task that looks like a number
	inst1 := NewInstance("123 is a magic number")
	inst1.ID = "inst-1"
	inst2 := NewInstance("Normal task")
	inst2.ID = "inst-2"
	session.Instances = []*Instance{inst1, inst2}

	// "1" should resolve to the first instance by sidebar number,
	// NOT by task name matching "123"
	result, err := orch.ResolveInstanceReference(session, "1")
	if err != nil {
		t.Errorf("ResolveInstanceReference('1') unexpected error: %v", err)
	}
	if result != inst1 {
		t.Errorf("ResolveInstanceReference('1') = %v, want %v (first instance by sidebar number)", result, inst1)
	}

	// "2" should resolve to the second instance by sidebar number
	result, err = orch.ResolveInstanceReference(session, "2")
	if err != nil {
		t.Errorf("ResolveInstanceReference('2') unexpected error: %v", err)
	}
	if result != inst2 {
		t.Errorf("ResolveInstanceReference('2') = %v, want %v (second instance by sidebar number)", result, inst2)
	}

	// "123" is an out-of-range sidebar number and should return a clear error
	// (it should NOT fall back to task name matching, which would be confusing)
	_, err = orch.ResolveInstanceReference(session, "123")
	if err == nil {
		t.Error("ResolveInstanceReference('123') should return error for out-of-range sidebar number")
	}

	// Non-numeric strings should still fall through to task name matching
	result, err = orch.ResolveInstanceReference(session, "magic")
	if err != nil {
		t.Errorf("ResolveInstanceReference('magic') unexpected error: %v", err)
	}
	if result != inst1 {
		t.Errorf("ResolveInstanceReference('magic') = %v, want %v (by task substring)", result, inst1)
	}
}
