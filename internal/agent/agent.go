// Package agent implements the LLM agent loop.
// It has no knowledge of UI or tool implementations — those are injected via Hooks.
package agent

import (
	"context"

	"github.com/ruiizy/elmi-harness/internal/api"
	"github.com/ruiizy/elmi-harness/provider"
)

// ToolResult is the outcome of a tool call returned by Hooks.OnToolUse.
type ToolResult struct {
	Content string
	IsError bool
}

// Hooks are the callbacks the agent calls during a run.
// Nil hooks are silently skipped.
type Hooks struct {
	// OnText is called for each streamed text chunk as it arrives.
	OnText func(chunk string)
	// OnToolUse is called for each tool call. The implementation handles
	// permission checking and execution. Returns the result to send back.
	OnToolUse func(name, input string) ToolResult
}

// Run executes the agent loop until the model stops requesting tools.
// Returns the updated message history and any API error.
func Run(ctx context.Context, llm provider.Provider, messages []api.Message, tools []api.ToolDef, hooks Hooks) ([]api.Message, error) {
	for {
		resp, err := llm.Stream(ctx, messages, tools, func(chunk string) {
			if hooks.OnText != nil {
				hooks.OnText(chunk)
			}
		})
		if err != nil {
			return messages, err
		}

		messages = append(messages, api.Message{
			Role:    api.RoleAssistant,
			Content: resp.Content,
		})

		var toolResults []api.Block
		for _, b := range resp.Content {
			if b.Type != api.BlockToolUse {
				continue // text was already delivered via onChunk
			}
			var result ToolResult
			if hooks.OnToolUse != nil {
				result = hooks.OnToolUse(b.ToolName, b.ToolInput)
			} else {
				result = ToolResult{Content: "no tool handler configured", IsError: true}
			}
			toolResults = append(toolResults, api.Block{
				Type:       api.BlockToolResult,
				ToolUseID:  b.ToolUseID,
				ToolResult: result.Content,
				IsError:    result.IsError,
			})
		}

		if resp.StopReason != api.StopToolUse {
			return messages, nil
		}
		messages = append(messages, api.Message{
			Role:    api.RoleUser,
			Content: toolResults,
		})
	}
}
