package tui

import "testing"

func TestCalculateContentDimensions(t *testing.T) {
	tests := []struct {
		name       string
		termWidth  int
		termHeight int
		wantWidth  int
		wantHeight int
	}{
		{
			name:       "standard terminal uses default sidebar width",
			termWidth:  120,
			termHeight: 40,
			wantWidth:  120 - DefaultSidebarWidth - ContentWidthOffset, // 120 - 36 - 7 = 77
			wantHeight: 40 - ContentHeightOffset,                       // 40 - 12 = 28
		},
		{
			name:       "narrow terminal uses minimum sidebar width",
			termWidth:  79,
			termHeight: 30,
			wantWidth:  79 - SidebarMinWidth - ContentWidthOffset, // 79 - 20 - 7 = 52
			wantHeight: 30 - ContentHeightOffset,                  // 30 - 12 = 18
		},
		{
			name:       "exactly 80 width uses default sidebar",
			termWidth:  80,
			termHeight: 24,
			wantWidth:  80 - DefaultSidebarWidth - ContentWidthOffset, // 80 - 36 - 7 = 37
			wantHeight: 24 - ContentHeightOffset,                      // 24 - 12 = 12
		},
		{
			name:       "very narrow terminal",
			termWidth:  60,
			termHeight: 20,
			wantWidth:  60 - SidebarMinWidth - ContentWidthOffset, // 60 - 20 - 7 = 33
			wantHeight: 20 - ContentHeightOffset,                  // 20 - 12 = 8
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWidth, gotHeight := CalculateContentDimensions(tt.termWidth, tt.termHeight)
			if gotWidth != tt.wantWidth {
				t.Errorf("CalculateContentDimensions() width = %v, want %v", gotWidth, tt.wantWidth)
			}
			if gotHeight != tt.wantHeight {
				t.Errorf("CalculateContentDimensions() height = %v, want %v", gotHeight, tt.wantHeight)
			}
		})
	}
}

func TestCalculateContentDimensionsWithSidebarWidth(t *testing.T) {
	tests := []struct {
		name         string
		termWidth    int
		termHeight   int
		sidebarWidth int
		wantWidth    int
		wantHeight   int
	}{
		{
			name:         "custom sidebar width 40",
			termWidth:    120,
			termHeight:   40,
			sidebarWidth: 40,
			wantWidth:    120 - 40 - ContentWidthOffset, // 120 - 40 - 7 = 73
			wantHeight:   40 - ContentHeightOffset,      // 40 - 12 = 28
		},
		{
			name:         "narrow terminal ignores custom width",
			termWidth:    79,
			termHeight:   30,
			sidebarWidth: 50,
			wantWidth:    79 - SidebarMinWidth - ContentWidthOffset, // 79 - 20 - 7 = 52
			wantHeight:   30 - ContentHeightOffset,                  // 30 - 12 = 18
		},
		{
			name:         "custom width clamped to minimum",
			termWidth:    120,
			termHeight:   40,
			sidebarWidth: 10,                                         // Below minimum
			wantWidth:    120 - SidebarMinWidth - ContentWidthOffset, // 120 - 20 - 7 = 93
			wantHeight:   40 - ContentHeightOffset,
		},
		{
			name:         "custom width clamped to maximum",
			termWidth:    120,
			termHeight:   40,
			sidebarWidth: 100,                                        // Above maximum
			wantWidth:    120 - SidebarMaxWidth - ContentWidthOffset, // 120 - 60 - 7 = 53
			wantHeight:   40 - ContentHeightOffset,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWidth, gotHeight := CalculateContentDimensionsWithSidebarWidth(tt.termWidth, tt.termHeight, tt.sidebarWidth)
			if gotWidth != tt.wantWidth {
				t.Errorf("CalculateContentDimensionsWithSidebarWidth() width = %v, want %v", gotWidth, tt.wantWidth)
			}
			if gotHeight != tt.wantHeight {
				t.Errorf("CalculateContentDimensionsWithSidebarWidth() height = %v, want %v", gotHeight, tt.wantHeight)
			}
		})
	}
}

func TestCalculateEffectiveSidebarWidth(t *testing.T) {
	tests := []struct {
		name      string
		termWidth int
		want      int
	}{
		{
			name:      "standard terminal returns default width",
			termWidth: 120,
			want:      DefaultSidebarWidth,
		},
		{
			name:      "narrow terminal returns minimum width",
			termWidth: 79,
			want:      SidebarMinWidth,
		},
		{
			name:      "exactly 80 returns default width",
			termWidth: 80,
			want:      DefaultSidebarWidth,
		},
		{
			name:      "very narrow terminal",
			termWidth: 40,
			want:      SidebarMinWidth,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateEffectiveSidebarWidth(tt.termWidth)
			if got != tt.want {
				t.Errorf("CalculateEffectiveSidebarWidth(%d) = %v, want %v", tt.termWidth, got, tt.want)
			}
		})
	}
}

func TestCalculateEffectiveSidebarWidthWithConfig(t *testing.T) {
	tests := []struct {
		name         string
		termWidth    int
		sidebarWidth int
		want         int
	}{
		{
			name:         "standard terminal with custom width",
			termWidth:    120,
			sidebarWidth: 45,
			want:         45,
		},
		{
			name:         "narrow terminal ignores custom width",
			termWidth:    79,
			sidebarWidth: 45,
			want:         SidebarMinWidth,
		},
		{
			name:         "custom width below minimum gets clamped",
			termWidth:    120,
			sidebarWidth: 15,
			want:         SidebarMinWidth,
		},
		{
			name:         "custom width above maximum gets clamped",
			termWidth:    120,
			sidebarWidth: 80,
			want:         SidebarMaxWidth,
		},
		{
			name:         "exactly 80 width uses custom",
			termWidth:    80,
			sidebarWidth: 30,
			want:         30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateEffectiveSidebarWidthWithConfig(tt.termWidth, tt.sidebarWidth)
			if got != tt.want {
				t.Errorf("CalculateEffectiveSidebarWidthWithConfig(%d, %d) = %v, want %v", tt.termWidth, tt.sidebarWidth, got, tt.want)
			}
		})
	}
}

func TestClampSidebarWidth(t *testing.T) {
	tests := []struct {
		name  string
		width int
		want  int
	}{
		{
			name:  "within bounds",
			width: 40,
			want:  40,
		},
		{
			name:  "at minimum",
			width: SidebarMinWidth,
			want:  SidebarMinWidth,
		},
		{
			name:  "at maximum",
			width: SidebarMaxWidth,
			want:  SidebarMaxWidth,
		},
		{
			name:  "below minimum",
			width: 10,
			want:  SidebarMinWidth,
		},
		{
			name:  "above maximum",
			width: 100,
			want:  SidebarMaxWidth,
		},
		{
			name:  "negative value",
			width: -5,
			want:  SidebarMinWidth,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampSidebarWidth(tt.width)
			if got != tt.want {
				t.Errorf("ClampSidebarWidth(%d) = %v, want %v", tt.width, got, tt.want)
			}
		})
	}
}

func TestLayoutConstants(t *testing.T) {
	// Verify constants have expected values to catch accidental changes
	if DefaultSidebarWidth != 36 {
		t.Errorf("DefaultSidebarWidth = %d, want 36", DefaultSidebarWidth)
	}
	if SidebarMinWidth != 20 {
		t.Errorf("SidebarMinWidth = %d, want 20", SidebarMinWidth)
	}
	if SidebarMaxWidth != 60 {
		t.Errorf("SidebarMaxWidth = %d, want 60", SidebarMaxWidth)
	}
	if ContentWidthOffset != 7 {
		t.Errorf("ContentWidthOffset = %d, want 7", ContentWidthOffset)
	}
	if ContentHeightOffset != 12 {
		t.Errorf("ContentHeightOffset = %d, want 12", ContentHeightOffset)
	}
}
