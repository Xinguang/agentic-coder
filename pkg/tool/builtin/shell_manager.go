// Package builtin provides built-in tools
package builtin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ShellState represents the state of a background shell
type ShellState string

const (
	ShellStateRunning   ShellState = "running"
	ShellStateCompleted ShellState = "completed"
	ShellStateFailed    ShellState = "failed"
	ShellStateKilled    ShellState = "killed"
)

// BackgroundShell represents a running background shell process
type BackgroundShell struct {
	ID          string
	Command     string
	Description string
	StartTime   time.Time
	EndTime     *time.Time
	State       ShellState
	ExitCode    int
	Output      *bytes.Buffer
	Error       error

	cmd    *exec.Cmd
	cancel context.CancelFunc
	mu     sync.Mutex
}

// ShellManager manages background shell processes
type ShellManager struct {
	shells    map[string]*BackgroundShell
	shellPath string
	mu        sync.RWMutex
}

// NewShellManager creates a new shell manager
func NewShellManager() *ShellManager {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	return &ShellManager{
		shells:    make(map[string]*BackgroundShell),
		shellPath: shell,
	}
}

// StartBackground starts a command in the background
func (m *ShellManager) StartBackground(command, description, cwd string) (*BackgroundShell, error) {
	id := uuid.New().String()[:8]

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, m.shellPath, "-c", command)
	cmd.Dir = cwd
	cmd.Env = os.Environ()

	output := &bytes.Buffer{}
	cmd.Stdout = output
	cmd.Stderr = output

	shell := &BackgroundShell{
		ID:          id,
		Command:     command,
		Description: description,
		StartTime:   time.Now(),
		State:       ShellStateRunning,
		Output:      output,
		cmd:         cmd,
		cancel:      cancel,
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	m.mu.Lock()
	m.shells[id] = shell
	m.mu.Unlock()

	// Monitor completion in background
	go m.monitorShell(shell)

	return shell, nil
}

// monitorShell monitors a background shell until completion
func (m *ShellManager) monitorShell(shell *BackgroundShell) {
	err := shell.cmd.Wait()
	now := time.Now()

	shell.mu.Lock()
	shell.EndTime = &now

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			shell.ExitCode = exitErr.ExitCode()
			shell.State = ShellStateFailed
		} else if shell.State == ShellStateKilled {
			// Already marked as killed
		} else {
			shell.Error = err
			shell.State = ShellStateFailed
		}
	} else {
		shell.ExitCode = 0
		shell.State = ShellStateCompleted
	}
	shell.mu.Unlock()
}

// Kill kills a background shell by ID
func (m *ShellManager) Kill(id string) error {
	m.mu.RLock()
	shell, exists := m.shells[id]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("shell '%s' not found", id)
	}

	shell.mu.Lock()
	defer shell.mu.Unlock()

	if shell.State != ShellStateRunning {
		return fmt.Errorf("shell '%s' is not running (state: %s)", id, shell.State)
	}

	shell.State = ShellStateKilled
	shell.cancel()

	// Give it a moment to terminate gracefully
	time.Sleep(100 * time.Millisecond)

	// Force kill if still running
	if shell.cmd.Process != nil {
		_ = shell.cmd.Process.Kill()
	}

	return nil
}

// Get returns a background shell by ID
func (m *ShellManager) Get(id string) *BackgroundShell {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.shells[id]
}

// List returns all background shells
func (m *ShellManager) List() []*BackgroundShell {
	m.mu.RLock()
	defer m.mu.RUnlock()

	shells := make([]*BackgroundShell, 0, len(m.shells))
	for _, shell := range m.shells {
		shells = append(shells, shell)
	}
	return shells
}

// ListRunning returns all running background shells
func (m *ShellManager) ListRunning() []*BackgroundShell {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var running []*BackgroundShell
	for _, shell := range m.shells {
		shell.mu.Lock()
		if shell.State == ShellStateRunning {
			running = append(running, shell)
		}
		shell.mu.Unlock()
	}
	return running
}

// GetOutput returns the current output of a background shell
func (m *ShellManager) GetOutput(id string, maxLen int) (string, ShellState, error) {
	m.mu.RLock()
	shell, exists := m.shells[id]
	m.mu.RUnlock()

	if !exists {
		return "", "", fmt.Errorf("shell '%s' not found", id)
	}

	shell.mu.Lock()
	defer shell.mu.Unlock()

	output := shell.Output.String()
	if maxLen > 0 && len(output) > maxLen {
		output = output[len(output)-maxLen:]
	}

	return output, shell.State, nil
}

// Remove removes a completed shell from tracking
func (m *ShellManager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	shell, exists := m.shells[id]
	if !exists {
		return fmt.Errorf("shell '%s' not found", id)
	}

	shell.mu.Lock()
	state := shell.State
	shell.mu.Unlock()

	if state == ShellStateRunning {
		return fmt.Errorf("cannot remove running shell '%s'", id)
	}

	delete(m.shells, id)
	return nil
}

// Cleanup removes all completed shells
func (m *ShellManager) Cleanup() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for id, shell := range m.shells {
		shell.mu.Lock()
		state := shell.State
		shell.mu.Unlock()

		if state != ShellStateRunning {
			delete(m.shells, id)
			count++
		}
	}
	return count
}

// ReadOutput implements io.Reader for streaming output
func (s *BackgroundShell) ReadOutput() io.Reader {
	return s.Output
}

// Duration returns the duration the shell has been running
func (s *BackgroundShell) Duration() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.EndTime != nil {
		return s.EndTime.Sub(s.StartTime)
	}
	return time.Since(s.StartTime)
}
