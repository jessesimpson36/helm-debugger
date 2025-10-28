package dlvcontroller

import (
	"bufio"
	"context"
	"fmt"
	"github.com/go-delve/delve/service/api"
	"github.com/go-delve/delve/service/rpc2"
	"strconv"
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

func (r *RPCDlvController) Configure(ctx context.Context, client *rpc2.RPCClient) error {
	condStartRequestedBreakpoint := &api.Breakpoint{
		Name: "conditionalstart",
		File: "text/template/exec.go",
		Line: 300,
	}
	condTrueRequestedBreakpoint := &api.Breakpoint{
		Name: "conditionalevaluatedtrue",
		File: "text/template/exec.go",
		Line: 307,
	}
	condFalseRequestedBreakpoint := &api.Breakpoint{
		Name: "conditionalevaluatedfalse",
		File: "text/template/exec.go",
		Line: 313,
	}

	breakpoints := []*api.Breakpoint{
		condStartRequestedBreakpoint,
		condTrueRequestedBreakpoint,
		condFalseRequestedBreakpoint,
	}
	for _, bp := range breakpoints {
		_, err := client.CreateBreakpoint(bp)
		if err != nil {
			return fmt.Errorf("Failed to create breakpoint: %w", err)
		}
	}
	return nil
}

func ReadOneLine(fileName string, lineNumber int) (string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	currentLine := 0
	returned := ""

	for scanner.Scan() {
		if currentLine == lineNumber-1 {
			returned = scanner.Text()
			fmt.Println(returned)
			break
		}
		currentLine++
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading file:", err)
	}
	return returned, nil
}

func (r *RPCDlvController) DisplayVars(client *rpc2.RPCClient) error {
	loadConfig := api.LoadConfig{
		FollowPointers:     true,
		MaxVariableRecurse: 10,
		MaxStringLen:       10000,
		MaxArrayValues:     10000,
		MaxStructFields:    -1,
	}
	parseName, err := client.EvalVariable(api.EvalScope{GoroutineID: 1, Frame: 0}, "pipe.tr.ParseName", loadConfig)
	if err != nil {
		//return fmt.Errorf("Failed to eval variable: %w", err)
		println(fmt.Errorf("Failed to eval variable: %w", err).Error())
	}
	name, err := client.EvalVariable(api.EvalScope{GoroutineID: 1, Frame: 0}, "pipe.tr.Name", loadConfig)
	if err != nil {
		//return fmt.Errorf("Failed to eval variable: %w", err)
		println(fmt.Errorf("Failed to eval variable: %w", err).Error())
	}
	line, err := client.EvalVariable(api.EvalScope{GoroutineID: 1, Frame: 0}, "pipe.Line", loadConfig)
	if err != nil {
		//return fmt.Errorf("Failed to eval variable: %w", err)
		println(fmt.Errorf("Failed to eval variable: %w", err).Error())
	}
	if parseName == nil || name == nil || line == nil {
		return nil
	}
	fmt.Println("ParseName:", parseName.Value)
	fmt.Println("Name:", name.Value)
	fmt.Println("Line:", line.Value)

	// read line number line.Value of file name.Value and print the line
	lineNumber, err := strconv.Atoi(line.Value)
	if err != nil {
		return fmt.Errorf("Failed to convert line number to int: %w", err)
	}
	ReadOneLine(name.Value, lineNumber)


//	vars, err := client.ListLocalVariables(api.EvalScope{GoroutineID: 1, Frame: 0}, loadConfig)
//	if err != nil {
//		return fmt.Errorf("Failed to list local variables: %w", err)
//	}
//	for _, v := range vars {
//		fmt.Printf("%s: %s = %s\n", v.Name, v.Type, v.Value)
//	}

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
