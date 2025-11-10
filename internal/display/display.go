package display

import (
	"bufio"
	"fmt"
	"github.com/go-delve/delve/service/api"
	"github.com/go-delve/delve/service/rpc2"
	"os"
)

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
			// fmt.Println(returned)
			break
		}
		currentLine++
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading file:", err)
	}
	return returned, nil
}

func DisplayAllLocal(client *rpc2.RPCClient) error {
	loadConfig := api.LoadConfig{
		FollowPointers:     true,
		MaxVariableRecurse: 10,
		MaxStringLen:       10000,
		MaxArrayValues:     10000,
		MaxStructFields:    -1,
	}
	vars, err := client.ListLocalVariables(api.EvalScope{GoroutineID: 1, Frame: 0}, loadConfig)
	if err != nil {
		return fmt.Errorf("Failed to list local variables: %w", err)
	}
	for _, v := range vars {
		fmt.Printf("%s: %s = %s\n", v.Name, v.Type, v.Value)
	}
	return nil
}
