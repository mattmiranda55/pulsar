package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	a.loadProjects()
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
	data, err := os.ReadFile(a.getConfigPath())
	if err != nil {
		return
	}
	json.Unmarshal(data, &a.projects)
}

// saveProjects saves projects to config file
func (a *App) saveProjects() {
	data, _ := json.MarshalIndent(a.projects, "", "  ")
	os.WriteFile(a.getConfigPath(), data, 0644)
}

// GetProjects returns all saved projects
func (a *App) GetProjects() []Project {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.projects
}

// SelectDirectory opens a directory picker dialog
func (a *App) SelectDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Laravel Project",
	})
}

// AddProject adds a new Laravel project
func (a *App) AddProject(name, path string) (Project, error) {
	// Validate it's a Laravel project
	artisanPath := filepath.Join(path, "artisan")
	if _, err := os.Stat(artisanPath); os.IsNotExist(err) {
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
	return project, nil
}

// RemoveProject removes a project by ID
func (a *App) RemoveProject(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i, p := range a.projects {
		if p.ID == id {
			a.projects = append(a.projects[:i], a.projects[i+1:]...)
			break
		}
	}
	a.saveProjects()
}

// RunTinker executes code through php artisan tinker
func (a *App) RunTinker(projectPath, code string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Verify project path
	artisanPath := filepath.Join(projectPath, "artisan")
	if _, err := os.Stat(artisanPath); os.IsNotExist(err) {
		return "Error: Invalid Laravel project path"
	}

	// Write code to temp file (without <?php tag as tinker doesn't need it)
	cleanCode := strings.TrimPrefix(strings.TrimSpace(code), "<?php")
	cleanCode = strings.TrimSpace(cleanCode)

	// Run tinker with the code file
	cmd := exec.CommandContext(ctx, "php", "artisan", "tinker", "--execute", cleanCode)
	cmd.Dir = projectPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// If --execute doesn't work, try piping to tinker
	if err != nil || (stdout.Len() == 0 && stderr.Len() == 0) {
		cmd = exec.CommandContext(ctx, "php", "artisan", "tinker")
		cmd.Dir = projectPath

		stdin, err := cmd.StdinPipe()
		if err != nil {
			return fmt.Sprintf("Error: %s", err)
		}

		stdout.Reset()
		stderr.Reset()
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Start(); err != nil {
			return fmt.Sprintf("Error starting tinker: %s", err)
		}

		// Write code and exit command
		io.WriteString(stdin, cleanCode+"\n")
		io.WriteString(stdin, "exit\n")
		stdin.Close()

		cmd.Wait()
	}

	var result strings.Builder

	// Parse and clean output
	output := stdout.String()
	// Remove tinker prompt artifacts
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Skip prompt lines and empty lines
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, ">>>") ||
			strings.HasPrefix(trimmed, "...") ||
			trimmed == "" ||
			strings.Contains(trimmed, "Psy Shell") ||
			strings.Contains(trimmed, "exit") {
			continue
		}
		// Remove "= " prefix from results
		if strings.HasPrefix(trimmed, "= ") {
			trimmed = strings.TrimPrefix(trimmed, "= ")
		}
		result.WriteString(trimmed + "\n")
	}

	if stderr.Len() > 0 {
		errOutput := stderr.String()
		// Filter out common non-error output
		if !strings.Contains(errOutput, "Xdebug:") {
			result.WriteString(errOutput)
		}
	}

	if ctx.Err() == context.DeadlineExceeded {
		return "Error: Execution timed out (60s limit)"
	}

	finalResult := strings.TrimSpace(result.String())
	if finalResult == "" {
		return "null"
	}
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
