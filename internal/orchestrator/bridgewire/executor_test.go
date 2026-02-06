package bridgewire

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/pipeline"
)

func TestNewPipelineExecutor_Validation(t *testing.T) {
	bus := event.NewBus()

	tests := []struct {
		name    string
		cfg     PipelineExecutorConfig
		wantErr string
	}{
		{
			name:    "missing orchestrator",
			cfg:     PipelineExecutorConfig{},
			wantErr: "Orchestrator is required",
		},
		{
			name: "missing session",
			cfg: PipelineExecutorConfig{
				Orchestrator: &orchestrator.Orchestrator{},
			},
			wantErr: "Session is required",
		},
		{
			name: "missing verifier",
			cfg: PipelineExecutorConfig{
				Orchestrator: &orchestrator.Orchestrator{},
				Session:      &orchestrator.Session{},
			},
			wantErr: "Verifier is required",
		},
		{
			name: "missing bus",
			cfg: PipelineExecutorConfig{
				Orchestrator: &orchestrator.Orchestrator{},
				Session:      &orchestrator.Session{},
				Verifier:     &mockVerifier{},
			},
			wantErr: "Bus is required",
		},
		{
			name: "missing pipeline",
			cfg: PipelineExecutorConfig{
				Orchestrator: &orchestrator.Orchestrator{},
				Session:      &orchestrator.Session{},
				Verifier:     &mockVerifier{},
				Bus:          bus,
			},
			wantErr: "Pipeline is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPipelineExecutor(tt.cfg)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestPipelineExecutor_DoubleStart(t *testing.T) {
	bus := event.NewBus()
	pe, err := NewPipelineExecutor(PipelineExecutorConfig{
		Orchestrator: &orchestrator.Orchestrator{},
		Session:      &orchestrator.Session{},
		Verifier:     &mockVerifier{},
		Bus:          bus,
		Pipeline:     &pipeline.Pipeline{},
	})
	if err != nil {
		t.Fatalf("NewPipelineExecutor: %v", err)
	}

	ctx := t.Context()
	if err := pe.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer pe.Stop()

	if err := pe.Start(ctx); err == nil {
		t.Error("second Start should return error")
	}
}

func TestPipelineExecutor_StopBeforeStart(t *testing.T) {
	bus := event.NewBus()
	pe, err := NewPipelineExecutor(PipelineExecutorConfig{
		Orchestrator: &orchestrator.Orchestrator{},
		Session:      &orchestrator.Session{},
		Verifier:     &mockVerifier{},
		Bus:          bus,
		Pipeline:     &pipeline.Pipeline{},
	})
	if err != nil {
		t.Fatalf("NewPipelineExecutor: %v", err)
	}
	pe.Stop() // should not panic
}

func TestPipelineExecutor_BridgesEmpty(t *testing.T) {
	bus := event.NewBus()
	pe, err := NewPipelineExecutor(PipelineExecutorConfig{
		Orchestrator: &orchestrator.Orchestrator{},
		Session:      &orchestrator.Session{},
		Verifier:     &mockVerifier{},
		Bus:          bus,
		Pipeline:     &pipeline.Pipeline{},
	})
	if err != nil {
		t.Fatalf("NewPipelineExecutor: %v", err)
	}

	bridges := pe.Bridges()
	if len(bridges) != 0 {
		t.Errorf("Bridges() = %d, want 0 before start", len(bridges))
	}
}
