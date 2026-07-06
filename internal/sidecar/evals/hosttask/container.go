package hosttask

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrDockerUnavailable is the typed error returned when the container engine
// cannot run containers (binary missing or daemon unreachable). The eval
// layer (S2+) translates it into a BLOCKED anomaly; S1 callers check it with
// errors.Is.
var ErrDockerUnavailable = errors.New("docker unavailable")

// ContainerSpec describes one grader container: the fixture's pinned image
// with the trial workspace bind-mounted read-write at /workspace, which is
// also the working directory of every command.
type ContainerSpec struct {
	// Image is the fixture's pinned container image.
	Image string
	// WorkspaceDir is the host workspace directory to bind-mount.
	WorkspaceDir string
	// Network is the docker network mode (Fixture.NetworkMode; "none" by
	// default — hermetic grading).
	Network string
}

// ExecResult is one command execution inside a container: the shell's exit
// code and its combined output (capped; see capTail).
type ExecResult struct {
	// ExitCode is the command's exit status (0 = pass).
	ExitCode int
	// Output is the combined stdout+stderr, tail-capped at maxOutputBytes.
	Output string
}

// ContainerRuntime abstracts the container engine so grader logic is
// testable without docker. The production implementation is DockerCLI.
type ContainerRuntime interface {
	// Available reports whether the engine can run containers, returning an
	// error wrapping ErrDockerUnavailable when it cannot.
	Available(ctx context.Context) error
	// Start launches a long-lived idle container for spec and returns its ID.
	Start(ctx context.Context, spec ContainerSpec) (string, error)
	// Exec runs one /bin/sh command inside the container. A non-zero command
	// exit is a normal ExecResult, not an error; errors mean the engine
	// itself failed.
	Exec(ctx context.Context, containerID, command string) (ExecResult, error)
	// Remove force-removes the container.
	Remove(ctx context.Context, containerID string) error
}

// maxOutputBytes caps per-command retained output so results stay
// artifact-sized; the tail is kept because failures conclude there.
const maxOutputBytes = 64 << 10

// capTail returns s, keeping only the last maxOutputBytes with a truncation
// marker when it overflows.
func capTail(s string) string {
	if len(s) <= maxOutputBytes {
		return s
	}
	return "[... output truncated ...]\n" + s[len(s)-maxOutputBytes:]
}

// DockerCLI is the production ContainerRuntime, driving the docker CLI. One
// grade attempt is one container: started idle, commands docker-exec'd in
// order (so setup effects persist across commands), then force-removed.
type DockerCLI struct {
	// Bin is the docker binary name; empty means "docker".
	Bin string
}

// bin returns the docker binary to invoke.
func (d DockerCLI) bin() string {
	if d.Bin == "" {
		return "docker"
	}
	return d.Bin
}

// Available implements ContainerRuntime: the binary must be on PATH and the
// daemon reachable, otherwise an ErrDockerUnavailable-wrapping error.
func (d DockerCLI) Available(ctx context.Context) error {
	if _, err := exec.LookPath(d.bin()); err != nil {
		return fmt.Errorf("%w: %q not found on PATH", ErrDockerUnavailable, d.bin())
	}
	out, err := exec.CommandContext(ctx, d.bin(), "version", "--format", "{{.Server.Version}}").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: daemon not reachable: %s", ErrDockerUnavailable, strings.TrimSpace(string(out)))
	}
	return nil
}

// Start implements ContainerRuntime:
//
//	docker run -d --rm --network <net> -v <ws>:/workspace -w /workspace \
//	  --entrypoint /bin/sh <image> -c 'sleep 2147483647'
//
// The idle sleep keeps the container alive for the attempt's execs; images
// must provide /bin/sh (package doc contract).
func (d DockerCLI) Start(ctx context.Context, spec ContainerSpec) (string, error) {
	ws, err := filepath.Abs(spec.WorkspaceDir)
	if err != nil {
		return "", fmt.Errorf("resolve workspace dir: %w", err)
	}
	out, err := exec.CommandContext(ctx, d.bin(), "run", "-d", "--rm",
		"--network", spec.Network,
		"-v", ws+":/workspace",
		"-w", "/workspace",
		"--entrypoint", "/bin/sh",
		spec.Image, "-c", "sleep 2147483647",
	).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker run %s: %w\n%s", spec.Image, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// Exec implements ContainerRuntime via `docker exec <id> /bin/sh -c <cmd>`.
// The command's exit code is read from the real process state; docker-level
// failures that also use non-zero exits (125–127) are indistinguishable from
// a command exiting the same way — an accepted, documented ambiguity.
func (d DockerCLI) Exec(ctx context.Context, containerID, command string) (ExecResult, error) {
	out, err := exec.CommandContext(ctx, d.bin(), "exec", containerID, "/bin/sh", "-c", command).CombinedOutput()
	res := ExecResult{Output: capTail(string(out))}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
			return res, nil
		}
		return res, fmt.Errorf("docker exec: %w", err)
	}
	return res, nil
}

// Remove implements ContainerRuntime via `docker rm -f`.
func (d DockerCLI) Remove(ctx context.Context, containerID string) error {
	out, err := exec.CommandContext(ctx, d.bin(), "rm", "-f", containerID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker rm -f %s: %w\n%s", containerID, err, strings.TrimSpace(string(out)))
	}
	return nil
}
