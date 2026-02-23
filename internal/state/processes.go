package state

import (
	"fmt"
	"os/exec"
	"sync"
	"time"
)

type ProcessInfo struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	WorkDir   string    `json:"workdir"`
	StartedAt time.Time `json:"started_at"`
	Done      bool      `json:"done"`
	ExitCode  int       `json:"exit_code"`
	stdout    *safeBuffer
	stderr    *safeBuffer
	cmd       *exec.Cmd
}

type safeBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (b *safeBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

type ProcessStore struct {
	mu        sync.Mutex
	processes map[string]*ProcessInfo
	counter   int
}

func NewProcessStore() *ProcessStore {
	return &ProcessStore{
		processes: make(map[string]*ProcessInfo),
	}
}

func (ps *ProcessStore) Start(command string, workdir string, env []string) (string, error) {
	ps.mu.Lock()
	ps.counter++
	id := fmt.Sprintf("proc_%d", ps.counter)
	ps.mu.Unlock()

	cmd := exec.Command("sh", "-c", command)
	if workdir != "" {
		cmd.Dir = workdir
	}
	if len(env) > 0 {
		cmd.Env = env
	}

	stdoutBuf := &safeBuffer{}
	stderrBuf := &safeBuffer{}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	info := &ProcessInfo{
		ID:        id,
		Command:   command,
		WorkDir:   workdir,
		StartedAt: time.Now(),
		stdout:    stdoutBuf,
		stderr:    stderrBuf,
		cmd:       cmd,
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start process: %s", err.Error())
	}

	ps.mu.Lock()
	ps.processes[id] = info
	ps.mu.Unlock()

	go func() {
		err := cmd.Wait()
		info.Done = true
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				info.ExitCode = exitErr.ExitCode()
			} else {
				info.ExitCode = -1
			}
		}
	}()

	return id, nil
}

func (ps *ProcessStore) Exec(command string, workdir string, env []string, timeout time.Duration) (stdout string, stderr string, exitCode int, err error) {
	cmd := exec.Command("sh", "-c", command)
	if workdir != "" {
		cmd.Dir = workdir
	}
	if len(env) > 0 {
		cmd.Env = env
	}

	stdoutBuf := &safeBuffer{}
	stderrBuf := &safeBuffer{}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		return "", "", -1, fmt.Errorf("failed to start command: %s", err.Error())
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		exitCode = 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				return stdoutBuf.String(), stderrBuf.String(), -1, err
			}
		}
		return stdoutBuf.String(), stderrBuf.String(), exitCode, nil
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return stdoutBuf.String(), stderrBuf.String(), -1, fmt.Errorf("command timed out after %s", timeout)
	}
}

func (ps *ProcessStore) Status(id string) (*ProcessInfo, string, string, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	info, ok := ps.processes[id]
	if !ok {
		return nil, "", "", fmt.Errorf("process %q not found", id)
	}

	return info, info.stdout.String(), info.stderr.String(), nil
}

func (ps *ProcessStore) Kill(id string, signal string) error {
	ps.mu.Lock()
	info, ok := ps.processes[id]
	ps.mu.Unlock()

	if !ok {
		return fmt.Errorf("process %q not found", id)
	}

	if info.Done {
		return fmt.Errorf("process %q already exited", id)
	}

	if info.cmd.Process == nil {
		return fmt.Errorf("process %q has no OS process", id)
	}

	return info.cmd.Process.Kill()
}

func (ps *ProcessStore) List() []*ProcessInfo {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	result := make([]*ProcessInfo, 0, len(ps.processes))
	for _, info := range ps.processes {
		result = append(result, info)
	}
	return result
}
