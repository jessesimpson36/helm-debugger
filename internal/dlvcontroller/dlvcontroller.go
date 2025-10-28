package dlvcontroller

import (
	"context"
	"fmt"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"github.com/go-delve/delve/service/rpc2"
	"time"
	"os"
	"os/exec"
)

type DlvController interface {
	StartSession(ctx context.Context) (rpc2.RPCClient, error)
	SendCommand(command string) error
	ReceiveResponse() (string, error)
	QuitSession() error
}

// https://github.com/go-delve/delve/blob/master/Documentation/api/ClientHowto.md

type RPCDlvController struct{}

func (r *RPCDlvController) StartSession(ctx context.Context) (*rpc2.RPCClient, error) {
	// helm template . --show-only templates/deployment.yaml
	cmd := exec.CommandContext(ctx, "dlv", "exec", "--headless", "--listen", "localhost:10122", "./helm/bin/helm", "--", "template", "test", "--show-only", "templates/deployment.yaml")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("Failed to start dlv subprocess: %w", err)
	}
	time.Sleep(3 * time.Second) // wait for dlv to start

	return rpc2.NewClient("localhost:10122"), nil
}

func (r *RPCDlvController) Configure(ctx context.Context, client *rpc2.RPCClient, frame *frame.Frame) error {
	for _, bp := range frame.Breakpoints {
		_, err := client.CreateBreakpoint(bp)
		if err != nil {
			return fmt.Errorf("Failed to create breakpoint: %w", err)
		}
	}
	return nil
}


func (r *RPCDlvController) SendCommand(command string) error {
	return nil
}

func (r *RPCDlvController) ReceiveResponse() (string, error) {
	return "", nil
}

func (r *RPCDlvController) QuitSession(client *rpc2.RPCClient) error {
	kill := true
	err := client.Detach(kill)
	if err != nil {
		return fmt.Errorf("Failed to detach from dlv session: %w", err)
	}
	return err
}
