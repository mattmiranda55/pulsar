package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx      context.Context
	projects []Project
	mu       sync.RWMutex
	logMu    sync.Mutex
	logStop  context.CancelFunc
}

// Project represents a Laravel project
type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		projects: []Project{},
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	log.Println("[Startup] App starting...")
	a.loadProjects()
	log.Printf("[Startup] Loaded %d projects", len(a.projects))
}

// getConfigPath returns the path to the config file
func (a *App) getConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".pulsar")
	os.MkdirAll(configDir, 0755)
	return filepath.Join(configDir, "projects.json")
}

// loadProjects loads projects from config file
func (a *App) loadProjects() {
	configPath := a.getConfigPath()
	log.Printf("[LoadProjects] Reading from: %s", configPath)
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("[LoadProjects] Error reading config: %v", err)
		return
	}
	if err := json.Unmarshal(data, &a.projects); err != nil {
		log.Printf("[LoadProjects] Error parsing JSON: %v", err)
	}
}

// saveProjects saves projects to config file
func (a *App) saveProjects() {
	configPath := a.getConfigPath()
	data, err := json.MarshalIndent(a.projects, "", "  ")
	if err != nil {
		log.Printf("[SaveProjects] Error marshaling JSON: %v", err)
		return
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		log.Printf("[SaveProjects] Error writing config: %v", err)
		return
	}
	log.Printf("[SaveProjects] Saved %d projects to: %s", len(a.projects), configPath)
}

// GetProjects returns all saved projects
func (a *App) GetProjects() []Project {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.projects
}

// SelectDirectory opens a directory picker dialog
func (a *App) SelectDirectory() (string, error) {
	log.Println("[SelectDirectory] Opening directory picker...")
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Laravel Project",
	})
	if err != nil {
		log.Printf("[SelectDirectory] Error: %v", err)
		return "", err
	}
	log.Printf("[SelectDirectory] Selected: %s", path)
	return path, nil
}

// AddProject adds a new Laravel project
func (a *App) AddProject(name, path string) (Project, error) {
	log.Printf("[AddProject] Adding project: name=%s, path=%s", name, path)

	// Validate it's a Laravel project
	artisanPath := filepath.Join(path, "artisan")
	log.Printf("[AddProject] Checking for artisan at: %s", artisanPath)
	if _, err := os.Stat(artisanPath); os.IsNotExist(err) {
		log.Printf("[AddProject] Error: artisan not found at %s", artisanPath)
		return Project{}, fmt.Errorf("not a valid Laravel project: artisan file not found")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	project := Project{
		ID:   fmt.Sprintf("%d", time.Now().UnixNano()),
		Name: name,
		Path: path,
	}
	a.projects = append(a.projects, project)
	a.saveProjects()
	log.Printf("[AddProject] Successfully added project: %+v", project)
	return project, nil
}

// RemoveProject removes a project by ID
func (a *App) RemoveProject(id string) {
	log.Printf("[RemoveProject] Removing project with ID: %s", id)
	a.mu.Lock()
	defer a.mu.Unlock()

	for i, p := range a.projects {
		if p.ID == id {
			log.Printf("[RemoveProject] Found project: %+v", p)
			a.projects = append(a.projects[:i], a.projects[i+1:]...)
			break
		}
	}
	a.saveProjects()
	log.Printf("[RemoveProject] Project removed, %d projects remaining", len(a.projects))
}

// StartLogTail begins tailing the Laravel log file for the given project path.
// It streams new lines to the frontend via Wails events.
func (a *App) StartLogTail(projectPath string) (string, error) {
	log.Printf("[StartLogTail] Starting log tail for project: %s", projectPath)

	if projectPath == "" {
		return "", fmt.Errorf("project path is required")
	}

	logFilePath := filepath.Join(projectPath, "storage", "logs", "laravel.log")
	if err := os.MkdirAll(filepath.Dir(logFilePath), 0755); err != nil {
		return "", fmt.Errorf("unable to prepare log directory: %w", err)
	}

	// Stop any existing tail
	a.logMu.Lock()
	if a.logStop != nil {
		log.Println("[StartLogTail] Stopping existing log tail")
		a.logStop()
		a.logStop = nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.logStop = cancel
	a.logMu.Unlock()

	initial, err := a.readLogTail(logFilePath, 200)
	if err != nil {
		log.Printf("[StartLogTail] Error reading existing logs: %v", err)
		a.StopLogTail()
		return "", fmt.Errorf("unable to read log file: %w", err)
	}

	go a.streamLogFile(ctx, logFilePath)

	return initial, nil
}

// StopLogTail stops the active log tailing session.
func (a *App) StopLogTail() {
	a.logMu.Lock()
	defer a.logMu.Unlock()

	if a.logStop != nil {
		log.Println("[StopLogTail] Stopping log tail")
		a.logStop()
		a.logStop = nil
	}
}

// readLogTail returns the last maxLines from the log file to seed the viewer.
func (a *App) readLogTail(path string, maxLines int) (string, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > maxLines {
			lines = lines[1:]
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return strings.Join(lines, "\n"), nil
}

// streamLogFile follows the log file similar to `tail -f` and emits updates.
func (a *App) streamLogFile(ctx context.Context, path string) {
	log.Printf("[LogTail] Streaming log file: %s", path)

	file, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Printf("[LogTail] Error opening log file: %v", err)
		runtime.EventsEmit(a.ctx, "log:error", fmt.Sprintf("Unable to open log file: %v", err))
		return
	}
	defer file.Close()

	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		log.Printf("[LogTail] Error seeking to end: %v", err)
		runtime.EventsEmit(a.ctx, "log:error", fmt.Sprintf("Unable to read log file: %v", err))
		return
	}

	reader := bufio.NewReader(file)

	for {
		select {
		case <-ctx.Done():
			log.Println("[LogTail] Log tail cancelled")
			return
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if errors.Is(err, io.EOF) {
					time.Sleep(300 * time.Millisecond)
					continue
				}

				log.Printf("[LogTail] Error reading log file: %v", err)
				runtime.EventsEmit(a.ctx, "log:error", fmt.Sprintf("Error reading log file: %v", err))
				return
			}

			cleaned := strings.TrimRight(line, "\r\n")
			runtime.EventsEmit(a.ctx, "log:update", cleaned)
		}
	}
}

// RunTinker executes code through php artisan tinker
func (a *App) RunTinker(projectPath, code string) string {
	log.Printf("[RunTinker] Starting execution for project: %s", projectPath)
	log.Printf("[RunTinker] Code to execute:\n%s", code)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Verify project path
	artisanPath := filepath.Join(projectPath, "artisan")
	if _, err := os.Stat(artisanPath); os.IsNotExist(err) {
		log.Printf("[RunTinker] Error: Invalid project path, artisan not found at %s", artisanPath)
		return "Error: Invalid Laravel project path"
	}

	// Clean code (remove <?php tag as tinker doesn't need it)
	cleanCode := strings.TrimPrefix(strings.TrimSpace(code), "<?php")
	cleanCode = strings.TrimSpace(cleanCode)
	log.Printf("[RunTinker] Cleaned code: %s", cleanCode)

	// Write code to a temp file for more reliable execution
	tmpFile, err := os.CreateTemp("", "tinker-*.php")
	if err != nil {
		log.Printf("[RunTinker] Error creating temp file: %v", err)
		return fmt.Sprintf("Error: %s", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write the code to temp file (no <?php tag - tinker doesn't want it)
	tmpFile.WriteString(cleanCode + "\n")
	tmpFile.Close()
	log.Printf("[RunTinker] Wrote code to temp file: %s", tmpFile.Name())

	// Run tinker with the temp file using shell piping
	shellCmd := fmt.Sprintf("cat %s | php artisan tinker 2>&1", tmpFile.Name())
	cmd := exec.CommandContext(ctx, "bash", "-c", shellCmd)
	cmd.Dir = projectPath
	log.Printf("[RunTinker] Running command: %s", shellCmd)

	output, err := cmd.CombinedOutput()
	log.Printf("[RunTinker] Command output: %s, err: %v", string(output), err)

	if ctx.Err() == context.DeadlineExceeded {
		log.Println("[RunTinker] Error: Execution timed out")
		return "Error: Execution timed out (60s limit)"
	}

	// Parse and clean output
	var result strings.Builder
	lines := strings.Split(string(output), "\n")
	log.Printf("[RunTinker] Processing %d lines", len(lines))
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		log.Printf("[RunTinker] Line %d: raw=%q trimmed=%q", i, line, trimmed)

		// Strip leading prompt characters (> or .)
		cleaned := trimmed
		for strings.HasPrefix(cleaned, "> ") || strings.HasPrefix(cleaned, ". ") {
			cleaned = strings.TrimPrefix(cleaned, "> ")
			cleaned = strings.TrimPrefix(cleaned, ". ")
			cleaned = strings.TrimSpace(cleaned)
		}

		// Skip empty, shell info, and echoed code lines
		if cleaned == "" || cleaned == "." ||
			strings.Contains(cleaned, "Psy Shell") ||
			cleaned == "exit" {
			log.Printf("[RunTinker] Skipping line")
			continue
		}

		// Check if this is a result line (starts with "= ")
		if strings.HasPrefix(cleaned, "= ") {
			val := strings.TrimPrefix(cleaned, "= ")
			log.Printf("[RunTinker] Found result: %s", val)
			result.WriteString(val + "\n")
		} else if !strings.HasPrefix(trimmed, "> ") && !strings.HasPrefix(trimmed, ". ") {
			// Keep non-prompt output (like dump() output)
			log.Printf("[RunTinker] Keeping output: %s", cleaned)
			result.WriteString(cleaned + "\n")
		}
	}

	finalResult := strings.TrimSpace(result.String())
	if finalResult == "" {
		log.Println("[RunTinker] Result is empty, returning 'null'")
		return "null"
	}
	log.Printf("[RunTinker] Final result: %s", finalResult)
	return finalResult
}

// RunTinkerStreaming runs tinker with streaming output (for future use)
func (a *App) RunTinkerStreaming(projectPath, code string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "php", "artisan", "tinker")
	cmd.Dir = projectPath

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return fmt.Sprintf("Error: %s", err)
	}

	// Write code
	cleanCode := strings.TrimPrefix(strings.TrimSpace(code), "<?php")
	io.WriteString(stdin, strings.TrimSpace(cleanCode)+"\n")
	io.WriteString(stdin, "exit\n")
	stdin.Close()

	var result strings.Builder
	scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
	for scanner.Scan() {
		line := scanner.Text()
		// Filter tinker prompt noise
		if !strings.HasPrefix(line, ">>>") && !strings.HasPrefix(line, "...") {
			result.WriteString(line + "\n")
		}
	}

	cmd.Wait()
	return strings.TrimSpace(result.String())
}
