package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/ruiizy/elmi-harness/internal/api"
)

const maxReadFileBytes = 10 * 1024 * 1024 // 10 MB

// Execute runs the named tool with rawInput (JSON) and returns (output, isError).
func Execute(name, rawInput string) (string, bool) {
	switch name {
	case "bash":
		var in struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
			return err.Error(), true
		}
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "sh", "-c", in.Command).CombinedOutput()
		if err != nil {
			return fmt.Sprintf("%s\n[exit error: %v]", out, err), true
		}
		return string(out), false

	case "read_file":
		var in struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
			return err.Error(), true
		}
		info, err := os.Stat(in.Path)
		if err != nil {
			return err.Error(), true
		}
		if info.Size() > maxReadFileBytes {
			return fmt.Sprintf("file too large (%d bytes); max %d bytes", info.Size(), maxReadFileBytes), true
		}
		data, err := os.ReadFile(in.Path)
		if err != nil {
			return err.Error(), true
		}
		return string(data), false

	case "write_file":
		var in struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
			return err.Error(), true
		}
		if err := os.WriteFile(in.Path, []byte(in.Content), 0644); err != nil {
			return err.Error(), true
		}
		return "wrote " + in.Path, false

	default:
		return fmt.Sprintf("unknown tool: %s", name), true
	}
}

// Definitions returns the tool schemas that match the cases in Execute.
// Add a tool to both places together.
func Definitions() []api.ToolDef {
	return []api.ToolDef{
		{
			Name:        "bash",
			Description: "Run a shell command and return its combined stdout/stderr.",
			InputSchema: map[string]any{
				"command": map[string]any{"type": "string", "description": "The shell command to run."},
			},
			Required: []string{"command"},
		},
		{
			Name:        "read_file",
			Description: "Read the contents of a file at the given path.",
			InputSchema: map[string]any{
				"path": map[string]any{"type": "string", "description": "Filesystem path to read."},
			},
			Required: []string{"path"},
		},
		{
			Name:        "write_file",
			Description: "Write content to a file (creating or overwriting it).",
			InputSchema: map[string]any{
				"path":    map[string]any{"type": "string", "description": "Filesystem path to write."},
				"content": map[string]any{"type": "string", "description": "Content to write."},
			},
			Required: []string{"path", "content"},
		},
	}
}
