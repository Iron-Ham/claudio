package panel

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/charmbracelet/lipgloss"
)

// mockTheme implements Theme for testing purposes.
type mockTheme struct{}

func (m *mockTheme) Primary() lipgloss.Style     { return lipgloss.NewStyle() }
func (m *mockTheme) Secondary() lipgloss.Style   { return lipgloss.NewStyle() }
func (m *mockTheme) Muted() lipgloss.Style       { return lipgloss.NewStyle() }
func (m *mockTheme) Error() lipgloss.Style       { return lipgloss.NewStyle() }
func (m *mockTheme) Warning() lipgloss.Style     { return lipgloss.NewStyle() }
func (m *mockTheme) Surface() lipgloss.Style     { return lipgloss.NewStyle() }
func (m *mockTheme) Border() lipgloss.Style      { return lipgloss.NewStyle() }
func (m *mockTheme) DiffAdd() lipgloss.Style     { return lipgloss.NewStyle() }
func (m *mockTheme) DiffRemove() lipgloss.Style  { return lipgloss.NewStyle() }
func (m *mockTheme) DiffHeader() lipgloss.Style  { return lipgloss.NewStyle() }
func (m *mockTheme) DiffHunk() lipgloss.Style    { return lipgloss.NewStyle() }
func (m *mockTheme) DiffContext() lipgloss.Style { return lipgloss.NewStyle() }

// mockPanelRenderer implements PanelRenderer for testing.
type mockPanelRenderer struct {
	rendered string
	height   int
}

func (m *mockPanelRenderer) Render(state *RenderState) string { return m.rendered }
func (m *mockPanelRenderer) Height() int                      { return m.height }

func TestPanelRendererInterface(t *testing.T) {
	// Verify the interface can be implemented
	var renderer PanelRenderer = &mockPanelRenderer{
		rendered: "test output",
		height:   10,
	}

	state := &RenderState{
		Width:  80,
		Height: 24,
		Theme:  &mockTheme{},
	}

	if got := renderer.Render(state); got != "test output" {
		t.Errorf("Render() = %q, want %q", got, "test output")
	}
	if got := renderer.Height(); got != 10 {
		t.Errorf("Height() = %d, want %d", got, 10)
	}
}

func TestThemeInterface(t *testing.T) {
	// Verify the Theme interface can be implemented
	var theme Theme = &mockTheme{}

	// Just verify methods are callable - return values are empty styles
	_ = theme.Primary()
	_ = theme.Secondary()
	_ = theme.Muted()
	_ = theme.Error()
	_ = theme.Warning()
	_ = theme.Surface()
	_ = theme.Border()
	_ = theme.DiffAdd()
	_ = theme.DiffRemove()
	_ = theme.DiffHeader()
	_ = theme.DiffHunk()
	_ = theme.DiffContext()
}

func TestRenderState_Validate(t *testing.T) {
	tests := []struct {
		name    string
		state   *RenderState
		wantErr error
	}{
		{
			name: "valid state",
			state: &RenderState{
				Width:  80,
				Height: 24,
				Theme:  &mockTheme{},
			},
			wantErr: nil,
		},
		{
			name: "zero width",
			state: &RenderState{
				Width:  0,
				Height: 24,
				Theme:  &mockTheme{},
			},
			wantErr: ErrInvalidWidth,
		},
		{
			name: "negative width",
			state: &RenderState{
				Width:  -1,
				Height: 24,
				Theme:  &mockTheme{},
			},
			wantErr: ErrInvalidWidth,
		},
		{
			name: "zero height",
			state: &RenderState{
				Width:  80,
				Height: 0,
				Theme:  &mockTheme{},
			},
			wantErr: ErrInvalidHeight,
		},
		{
			name: "negative height",
			state: &RenderState{
				Width:  80,
				Height: -5,
				Theme:  &mockTheme{},
			},
			wantErr: ErrInvalidHeight,
		},
		{
			name: "nil theme",
			state: &RenderState{
				Width:  80,
				Height: 24,
				Theme:  nil,
			},
			wantErr: ErrNilTheme,
		},
		{
			name: "width checked before height",
			state: &RenderState{
				Width:  0,
				Height: 0,
				Theme:  nil,
			},
			wantErr: ErrInvalidWidth,
		},
		{
			name: "height checked before theme",
			state: &RenderState{
				Width:  80,
				Height: 0,
				Theme:  nil,
			},
			wantErr: ErrInvalidHeight,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.state.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRenderState_ValidateBasic(t *testing.T) {
	tests := []struct {
		name    string
		state   *RenderState
		wantErr error
	}{
		{
			name: "valid dimensions",
			state: &RenderState{
				Width:  80,
				Height: 24,
			},
			wantErr: nil,
		},
		{
			name: "nil theme is ok for basic validation",
			state: &RenderState{
				Width:  80,
				Height: 24,
				Theme:  nil,
			},
			wantErr: nil,
		},
		{
			name: "zero width fails",
			state: &RenderState{
				Width:  0,
				Height: 24,
			},
			wantErr: ErrInvalidWidth,
		},
		{
			name: "zero height fails",
			state: &RenderState{
				Width:  80,
				Height: 0,
			},
			wantErr: ErrInvalidHeight,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.state.ValidateBasic()
			if err != tt.wantErr {
				t.Errorf("ValidateBasic() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRenderState_IsActiveInstance(t *testing.T) {
	inst1 := &orchestrator.Instance{ID: "inst-1", Task: "Task 1"}
	inst2 := &orchestrator.Instance{ID: "inst-2", Task: "Task 2"}

	tests := []struct {
		name           string
		activeInstance *orchestrator.Instance
		checkInstance  *orchestrator.Instance
		want           bool
	}{
		{
			name:           "same instance is active",
			activeInstance: inst1,
			checkInstance:  inst1,
			want:           true,
		},
		{
			name:           "different instance is not active",
			activeInstance: inst1,
			checkInstance:  inst2,
			want:           false,
		},
		{
			name:           "nil active instance",
			activeInstance: nil,
			checkInstance:  inst1,
			want:           false,
		},
		{
			name:           "nil check instance",
			activeInstance: inst1,
			checkInstance:  nil,
			want:           false,
		},
		{
			name:           "both nil",
			activeInstance: nil,
			checkInstance:  nil,
			want:           false,
		},
		{
			name:           "same ID different objects",
			activeInstance: &orchestrator.Instance{ID: "inst-1"},
			checkInstance:  &orchestrator.Instance{ID: "inst-1"},
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &RenderState{
				ActiveInstance: tt.activeInstance,
			}
			if got := state.IsActiveInstance(tt.checkInstance); got != tt.want {
				t.Errorf("IsActiveInstance() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRenderState_InstanceCount(t *testing.T) {
	tests := []struct {
		name      string
		instances []*orchestrator.Instance
		want      int
	}{
		{
			name:      "nil instances",
			instances: nil,
			want:      0,
		},
		{
			name:      "empty instances",
			instances: []*orchestrator.Instance{},
			want:      0,
		},
		{
			name: "one instance",
			instances: []*orchestrator.Instance{
				{ID: "inst-1"},
			},
			want: 1,
		},
		{
			name: "multiple instances",
			instances: []*orchestrator.Instance{
				{ID: "inst-1"},
				{ID: "inst-2"},
				{ID: "inst-3"},
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &RenderState{Instances: tt.instances}
			if got := state.InstanceCount(); got != tt.want {
				t.Errorf("InstanceCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRenderState_GetInstance(t *testing.T) {
	inst1 := &orchestrator.Instance{ID: "inst-1"}
	inst2 := &orchestrator.Instance{ID: "inst-2"}
	instances := []*orchestrator.Instance{inst1, inst2}

	tests := []struct {
		name  string
		index int
		want  *orchestrator.Instance
	}{
		{
			name:  "first instance",
			index: 0,
			want:  inst1,
		},
		{
			name:  "second instance",
			index: 1,
			want:  inst2,
		},
		{
			name:  "negative index",
			index: -1,
			want:  nil,
		},
		{
			name:  "index out of bounds",
			index: 10,
			want:  nil,
		},
		{
			name:  "exact boundary",
			index: 2,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &RenderState{Instances: instances}
			got := state.GetInstance(tt.index)
			if got != tt.want {
				t.Errorf("GetInstance(%d) = %v, want %v", tt.index, got, tt.want)
			}
		})
	}
}

func TestRenderState_GetInstance_Empty(t *testing.T) {
	state := &RenderState{Instances: nil}
	if got := state.GetInstance(0); got != nil {
		t.Errorf("GetInstance(0) on empty state = %v, want nil", got)
	}
}

func TestRenderState_HasInstances(t *testing.T) {
	tests := []struct {
		name      string
		instances []*orchestrator.Instance
		want      bool
	}{
		{
			name:      "nil instances",
			instances: nil,
			want:      false,
		},
		{
			name:      "empty instances",
			instances: []*orchestrator.Instance{},
			want:      false,
		},
		{
			name: "has instances",
			instances: []*orchestrator.Instance{
				{ID: "inst-1"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &RenderState{Instances: tt.instances}
			if got := state.HasInstances(); got != tt.want {
				t.Errorf("HasInstances() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRenderState_VisibleRange(t *testing.T) {
	instances := make([]*orchestrator.Instance, 10)
	for i := range 10 {
		instances[i] = &orchestrator.Instance{ID: "inst"}
	}

	tests := []struct {
		name           string
		instances      []*orchestrator.Instance
		scrollOffset   int
		availableSlots int
		wantStart      int
		wantEnd        int
	}{
		{
			name:           "no scroll - fits exactly",
			instances:      instances,
			scrollOffset:   0,
			availableSlots: 10,
			wantStart:      0,
			wantEnd:        10,
		},
		{
			name:           "no scroll - partial view",
			instances:      instances,
			scrollOffset:   0,
			availableSlots: 5,
			wantStart:      0,
			wantEnd:        5,
		},
		{
			name:           "scrolled down",
			instances:      instances,
			scrollOffset:   3,
			availableSlots: 5,
			wantStart:      3,
			wantEnd:        8,
		},
		{
			name:           "scrolled to end",
			instances:      instances,
			scrollOffset:   5,
			availableSlots: 5,
			wantStart:      5,
			wantEnd:        10,
		},
		{
			name:           "scroll past end - clamped",
			instances:      instances,
			scrollOffset:   8,
			availableSlots: 5,
			wantStart:      8,
			wantEnd:        10,
		},
		{
			name:           "negative scroll - clamped to 0",
			instances:      instances,
			scrollOffset:   -5,
			availableSlots: 5,
			wantStart:      0,
			wantEnd:        5,
		},
		{
			name:           "scroll way past end",
			instances:      instances,
			scrollOffset:   100,
			availableSlots: 5,
			wantStart:      9,
			wantEnd:        10,
		},
		{
			name:           "empty instances",
			instances:      nil,
			scrollOffset:   0,
			availableSlots: 5,
			wantStart:      0,
			wantEnd:        0,
		},
		{
			name:           "zero available slots",
			instances:      instances,
			scrollOffset:   0,
			availableSlots: 0,
			wantStart:      0,
			wantEnd:        0,
		},
		{
			name:           "negative available slots",
			instances:      instances,
			scrollOffset:   0,
			availableSlots: -1,
			wantStart:      0,
			wantEnd:        0,
		},
		{
			name:           "more slots than instances",
			instances:      instances[:3],
			scrollOffset:   0,
			availableSlots: 10,
			wantStart:      0,
			wantEnd:        3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &RenderState{
				Instances:    tt.instances,
				ScrollOffset: tt.scrollOffset,
			}
			gotStart, gotEnd := state.VisibleRange(tt.availableSlots)
			if gotStart != tt.wantStart || gotEnd != tt.wantEnd {
				t.Errorf("VisibleRange(%d) = (%d, %d), want (%d, %d)",
					tt.availableSlots, gotStart, gotEnd, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestDefaultRenderState(t *testing.T) {
	state := DefaultRenderState()

	if state.Width != 80 {
		t.Errorf("DefaultRenderState() Width = %d, want 80", state.Width)
	}
	if state.Height != 24 {
		t.Errorf("DefaultRenderState() Height = %d, want 24", state.Height)
	}
	if state.Instances == nil {
		t.Error("DefaultRenderState() Instances should not be nil")
	}
	if len(state.Instances) != 0 {
		t.Errorf("DefaultRenderState() Instances length = %d, want 0", len(state.Instances))
	}
	if state.ActiveIndex != -1 {
		t.Errorf("DefaultRenderState() ActiveIndex = %d, want -1", state.ActiveIndex)
	}
	if state.Theme != nil {
		t.Error("DefaultRenderState() Theme should be nil (user must set)")
	}
}

func TestNewRenderState(t *testing.T) {
	state := NewRenderState(120, 40)

	if state.Width != 120 {
		t.Errorf("NewRenderState() Width = %d, want 120", state.Width)
	}
	if state.Height != 40 {
		t.Errorf("NewRenderState() Height = %d, want 40", state.Height)
	}
	if state.Instances == nil {
		t.Error("NewRenderState() Instances should not be nil")
	}
	if len(state.Instances) != 0 {
		t.Errorf("NewRenderState() Instances length = %d, want 0", len(state.Instances))
	}
	if state.ActiveIndex != -1 {
		t.Errorf("NewRenderState() ActiveIndex = %d, want -1", state.ActiveIndex)
	}
}

func TestRenderState_AllFieldsAccessible(t *testing.T) {
	inst1 := &orchestrator.Instance{ID: "inst-1", Task: "Task 1"}
	inst2 := &orchestrator.Instance{ID: "inst-2", Task: "Task 2"}
	theme := &mockTheme{}

	state := &RenderState{
		Width:          100,
		Height:         50,
		ActiveInstance: inst1,
		Instances:      []*orchestrator.Instance{inst1, inst2},
		Theme:          theme,
		ActiveIndex:    0,
		ScrollOffset:   5,
		Focused:        true,
	}

	// Verify all fields are accessible and have expected values
	if state.Width != 100 {
		t.Errorf("Width = %d, want 100", state.Width)
	}
	if state.Height != 50 {
		t.Errorf("Height = %d, want 50", state.Height)
	}
	if state.ActiveInstance != inst1 {
		t.Error("ActiveInstance mismatch")
	}
	if len(state.Instances) != 2 {
		t.Errorf("Instances length = %d, want 2", len(state.Instances))
	}
	if state.Theme != theme {
		t.Error("Theme mismatch")
	}
	if state.ActiveIndex != 0 {
		t.Errorf("ActiveIndex = %d, want 0", state.ActiveIndex)
	}
	if state.ScrollOffset != 5 {
		t.Errorf("ScrollOffset = %d, want 5", state.ScrollOffset)
	}
	if !state.Focused {
		t.Error("Focused = false, want true")
	}
}

func TestErrorConstants(t *testing.T) {
	// Verify error messages are meaningful
	if ErrInvalidWidth.Error() != "width must be positive" {
		t.Errorf("ErrInvalidWidth message unexpected: %s", ErrInvalidWidth.Error())
	}
	if ErrInvalidHeight.Error() != "height must be positive" {
		t.Errorf("ErrInvalidHeight message unexpected: %s", ErrInvalidHeight.Error())
	}
	if ErrNilTheme.Error() != "theme cannot be nil" {
		t.Errorf("ErrNilTheme message unexpected: %s", ErrNilTheme.Error())
	}
}
