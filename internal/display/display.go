package display

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-delve/delve/service/api"
	"github.com/go-delve/delve/service/rpc2"
)

// ReadOneLine reads a single 1-based line from fileName. When fileName is
// relative it is resolved against baseDir (which may be empty, meaning the
// current directory).
func ReadOneLine(baseDir, fileName string, lineNumber int) (string, error) {
	path := fileName
	if !filepath.IsAbs(path) && baseDir != "" {
		path = filepath.Join(baseDir, path)
	}

	file, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error opening file:", err)
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
