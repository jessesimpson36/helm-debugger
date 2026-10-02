package dlvcontroller

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/go-delve/delve/service/rpc2"
	"github.com/jessesimpson36/helm-debugger/internal/frame/delegate"
	"github.com/jessesimpson36/helm-debugger/internal/settings"
)

const (
	// startupTimeout is how long we wait for the headless delve server to
	// accept TCP connections before giving up.
	startupTimeout = 30 * time.Second
	// dialInterval is the delay between connection attempts.
	dialInterval = 100 * time.Millisecond
)

// Session wraps a running headless delve server and its RPC client.
type Session struct {
	Client *rpc2.RPCClient

	cmd    *exec.Cmd
	cancel context.CancelFunc
}

// Start launches a headless delve server debugging the configured helm binary
// running `helm template` with the configured arguments. It blocks until the
// server is accepting connections or the timeout elapses.
//
// Unlike rpc2.NewClient, it never calls log.Fatal on a connection failure, and
// it cleans up the subprocess when startup fails.
func Start(ctx context.Context, cfg *settings.Settings, log io.Writer) (*Session, error) {
	if cfg.CompiledHelmPath == "" {
		return nil, fmt.Errorf("helm binary path is required")
	}
	// dlv resolves the target binary relative to its own working directory,
	// which we change to cfg.WorkingDir, and it does not consult PATH. Resolve
	// the binary here (handling both PATH names and relative/explicit paths) so
	// dlv always receives an absolute path.
	helmBinary, err := exec.LookPath(cfg.CompiledHelmPath)
	if err != nil {
		return nil, fmt.Errorf("locating helm binary %q: %w", cfg.CompiledHelmPath, err)
	}
	// LookPath may return a relative path for inputs containing a slash; dlv
	// resolves that against its own working directory, so make it absolute.
	if !filepath.IsAbs(helmBinary) {
		abs, err := filepath.Abs(helmBinary)
		if err != nil {
			return nil, fmt.Errorf("resolving helm binary path: %w", err)
		}
		helmBinary = abs
	}

	port, err := choosePort(cfg.DebugPort)
	if err != nil {
		return nil, fmt.Errorf("selecting debug port: %w", err)
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	args := []string{
		"exec",
		"--headless",
		"--listen", addr,
		"--api-version", "2",
		helmBinary,
		"--",
		"template",
		cfg.ChartName,
	}
	args = append(args, cfg.CommandArgs...)

	runCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(runCtx, "dlv", args...)
	cmd.Dir = cfg.EffectiveWorkingDir()
	if log != nil {
		cmd.Stdout = log
		cmd.Stderr = log
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start dlv subprocess: %w", err)
	}

	client, err := dial(ctx, addr, startupTimeout)
	if err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		cancel()
		return nil, fmt.Errorf("delve server at %s did not become ready: %w", addr, err)
	}

	return &Session{Client: client, cmd: cmd, cancel: cancel}, nil
}

// dial connects to addr, retrying until the deadline elapses.
func dial(ctx context.Context, addr string, timeout time.Duration) (*rpc2.RPCClient, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			return rpc2.NewClientFromConn(conn), nil
		}
		lastErr = err
		time.Sleep(dialInterval)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timed out after %s", timeout)
	}
	return nil, lastErr
}

// choosePort returns requested when it is non-zero, otherwise it asks the OS
// for a free port.
func choosePort(requested int) (int, error) {
	if requested != 0 {
		return requested, nil
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// Configure installs all breakpoints for the supplied frames on the session.
func (s *Session) Configure(frames []*delegate.DelegateFrame) error {
	for _, f := range frames {
		for _, bp := range f.Breakpoints {
			if _, err := s.Client.CreateBreakpoint(bp); err != nil {
				return fmt.Errorf("failed to create breakpoint %s (%s:%d): %w", bp.Name, bp.File, bp.Line, err)
			}
		}
	}
	return nil
}

// Close detaches from the debuggee (killing it) and terminates the delve
// subprocess.
func (s *Session) Close() error {
	var detachErr error
	if s.Client != nil {
		detachErr = s.Client.Detach(true)
	}
	if s.cmd != nil && s.cmd.Process != nil {
		// The context cancellation in cancel() is what guarantees the process
		// is reaped even if detach failed.
		if s.cancel != nil {
			s.cancel()
		}
		_ = s.cmd.Process.Kill()
		_, _ = s.cmd.Process.Wait()
	} else if s.cancel != nil {
		s.cancel()
	}
	return detachErr
}
