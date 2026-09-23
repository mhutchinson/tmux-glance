package tmux

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// ProcessInspector inspects a process and its children, returning their command-line invocations.
type ProcessInspector func(ctx context.Context, pid int) ([]string, error)

// DefaultProcessInspector is the production process inspector.
var DefaultProcessInspector ProcessInspector = InspectProcessTree

// InspectProcessTree returns command lines for pid and any immediate child processes.
func InspectProcessTree(ctx context.Context, pid int) ([]string, error) {
	if pid <= 1 {
		return nil, nil
	}

	// 1. Linux /proc fast-path
	if cmdlineBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); err == nil {
		var results []string
		parentCmd := string(bytes.ReplaceAll(cmdlineBytes, []byte{0}, []byte{' '}))
		if strings.TrimSpace(parentCmd) != "" {
			results = append(results, strings.TrimSpace(parentCmd))
		}

		// Check children via /proc/<pid>/task/<pid>/children
		if childBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/task/%d/children", pid, pid)); err == nil {
			childPIDs := strings.Fields(string(childBytes))
			for _, cpidStr := range childPIDs {
				if cBytes, err := os.ReadFile(fmt.Sprintf("/proc/%s/cmdline", cpidStr)); err == nil {
					childCmd := string(bytes.ReplaceAll(cBytes, []byte{0}, []byte{' '}))
					if strings.TrimSpace(childCmd) != "" {
						results = append(results, strings.TrimSpace(childCmd))
					}
				}
			}
		}
		if len(results) > 0 {
			return results, nil
		}
	}

	// 2. POSIX / macOS fallback using ps
	psPath, err := exec.LookPath("ps")
	if err != nil {
		if _, errBin := os.Stat("/bin/ps"); errBin == nil {
			psPath = "/bin/ps"
		} else {
			return nil, nil
		}
	}

	cmd := exec.CommandContext(ctx, psPath, "-eo", "pid,ppid,command")
	out, err := cmd.Output()
	if err != nil {
		cmd = exec.CommandContext(ctx, psPath, "-eo", "pid,ppid,args")
		out, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("inspect process tree for pid %d: %w", pid, err)
		}
	}

	var results []string
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		curPID, err1 := strconv.Atoi(fields[0])
		curPPID, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			continue // skip header line (PID PPID COMMAND)
		}
		if curPID == pid || curPPID == pid {
			// Extract command part (reconstruct everything after ppid field)
			idx := strings.Index(line, fields[1])
			if idx != -1 {
				rem := strings.TrimSpace(line[idx+len(fields[1]):])
				if rem != "" {
					results = append(results, rem)
				}
			}
		}
	}

	return results, nil
}
