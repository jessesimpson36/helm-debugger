package display

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ResolveAndReadOneLine resolves a template filename (which may include a chart name prefix)
// to an actual file path and reads the specified line.
//
// Resolution order:
// 1. Try to open the file as-is
// 2. If the path has a chart prefix (e.g., "camunda-platform/templates/foo.yaml"):
//   - Search CWD recursively for Chart.yaml files matching the chart name
//   - Check the charts/ directory for decompressed subcharts
//   - Check the charts/ directory for .tgz archives
func ResolveAndReadOneLine(fileName string, lineNumber int) (string, error) {
	// 1. Try to open the file as-is
	if fileExists(fileName) {
		return ReadOneLine(fileName, lineNumber)
	}

	// 2. Parse the path to extract chart prefix and relative template path
	parts := strings.SplitN(fileName, "/", 2)
	if len(parts) < 2 {
		// No path separator, can't resolve further
		return "", fmt.Errorf("file not found and cannot resolve: %s", fileName)
	}

	chartPrefix := parts[0]
	templatePath := parts[1]

	// 3. Search CWD recursively for Chart.yaml files matching the chart name
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	resolvedPath, err := findChartAndResolvePath(cwd, chartPrefix, templatePath)
	if err == nil && fileExists(resolvedPath) {
		return ReadOneLine(resolvedPath, lineNumber)
	}

	// 4. Check the charts/ directory for decompressed subcharts
	chartsDir := filepath.Join(cwd, "charts")
	if dirExists(chartsDir) {
		// Try decompressed folder first (preferred)
		decompressedPath := filepath.Join(chartsDir, chartPrefix, templatePath)
		if fileExists(decompressedPath) {
			return ReadOneLine(decompressedPath, lineNumber)
		}

		// 5. Check for .tgz archives in charts/ directory
		tgzPath, innerPath, err := findTgzForChart(chartsDir, chartPrefix, templatePath)
		if err == nil {
			return ReadOneLineFromTgz(tgzPath, innerPath, lineNumber)
		}
	}

	// 6. As a last resort, try just the template path without the chart prefix
	if fileExists(templatePath) {
		return ReadOneLine(templatePath, lineNumber)
	}

	return "", fmt.Errorf("could not resolve file: %s (tried chart prefix '%s' with template path '%s')", fileName, chartPrefix, templatePath)
}

// findChartAndResolvePath searches for a Chart.yaml file whose name matches chartPrefix
// and returns the resolved path to the template file
func findChartAndResolvePath(rootDir string, chartPrefix string, templatePath string) (string, error) {
	var resolvedPath string
	found := false

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip directories we can't access
		}

		// Skip hidden directories and common non-chart directories
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Look for Chart.yaml files
		if d.Name() == "Chart.yaml" {
			chartName, err := parseChartNameFromFile(path)
			if err != nil {
				return nil // Skip Chart.yaml files we can't parse
			}

			if chartName == chartPrefix {
				// Found a matching chart, construct the resolved path
				chartDir := filepath.Dir(path)
				resolvedPath = filepath.Join(chartDir, templatePath)
				found = true
				return filepath.SkipAll // Stop walking
			}
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return "", err
	}

	if !found {
		return "", fmt.Errorf("no Chart.yaml found with name '%s'", chartPrefix)
	}

	return resolvedPath, nil
}

// parseChartNameFromFile reads a Chart.yaml file and extracts the chart name using regex
func parseChartNameFromFile(chartYamlPath string) (string, error) {
	file, err := os.Open(chartYamlPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Use regex to find the name field
	// Pattern matches "name: chartname" at the start of a line
	nameRegex := regexp.MustCompile(`(?m)^name:\s*(.+?)\s*$`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		matches := nameRegex.FindStringSubmatch(line)
		if len(matches) >= 2 {
			return strings.TrimSpace(matches[1]), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", fmt.Errorf("no 'name' field found in %s", chartYamlPath)
}

// findTgzForChart searches for a .tgz file in the charts directory that matches the chart name
// Returns the path to the .tgz file and the inner path within the archive
func findTgzForChart(chartsDir string, chartPrefix string, templatePath string) (tgzPath string, innerPath string, err error) {
	entries, err := os.ReadDir(chartsDir)
	if err != nil {
		return "", "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".tgz") {
			continue
		}

		// Check if this .tgz matches the chart name
		// Common patterns: "chartname-1.2.3.tgz", "chartname.tgz"
		baseName := strings.TrimSuffix(name, ".tgz")

		// Remove version suffix (e.g., "-1.2.3", "-0.1.0-alpha")
		// Version pattern: starts with a digit after the last hyphen
		if idx := strings.LastIndex(baseName, "-"); idx > 0 {
			potentialVersion := baseName[idx+1:]
			if len(potentialVersion) > 0 && potentialVersion[0] >= '0' && potentialVersion[0] <= '9' {
				baseName = baseName[:idx]
			}
		}

		if baseName == chartPrefix {
			tgzPath = filepath.Join(chartsDir, name)
			// The inner path in a helm .tgz is typically: chartname/templates/...
			innerPath = filepath.Join(chartPrefix, templatePath)
			return tgzPath, innerPath, nil
		}
	}

	return "", "", fmt.Errorf("no .tgz file found for chart '%s' in %s", chartPrefix, chartsDir)
}

// ReadOneLineFromTgz reads a specific line from a file inside a .tgz archive
func ReadOneLineFromTgz(tgzPath string, innerPath string, lineNumber int) (string, error) {
	file, err := os.Open(tgzPath)
	if err != nil {
		return "", fmt.Errorf("failed to open tgz file: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	// Normalize the inner path for comparison
	innerPath = filepath.ToSlash(innerPath)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Normalize the header name for comparison
		headerName := filepath.ToSlash(header.Name)

		if headerName == innerPath || strings.TrimPrefix(headerName, "./") == innerPath {
			// Found the file, read the specific line
			return readLineFromReader(tarReader, lineNumber)
		}
	}

	return "", fmt.Errorf("file '%s' not found in archive '%s'", innerPath, tgzPath)
}

// readLineFromReader reads a specific line from an io.Reader
func readLineFromReader(reader io.Reader, lineNumber int) (string, error) {
	scanner := bufio.NewScanner(reader)
	currentLine := 0

	for scanner.Scan() {
		currentLine++
		if currentLine == lineNumber {
			return scanner.Text(), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading from archive: %w", err)
	}

	return "", fmt.Errorf("line %d not found (file has only %d lines)", lineNumber, currentLine)
}

// fileExists checks if a file exists and is not a directory
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// dirExists checks if a directory exists
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
