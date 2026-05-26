package agent_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ruiizy/elmi-harness/internal/agent"
	"github.com/ruiizy/elmi-harness/internal/api"
)

// mockProvider implements provider.Provider with scripted responses.
type mockProvider struct {
	responses []api.Response
	idx       int
	calls     [][]api.Message // messages received per Send call
}

func (m *mockProvider) Stream(_ context.Context, messages []api.Message, _ []api.ToolDef, onChunk func(string)) (api.Response, error) {
	if m.idx >= len(m.responses) {
		return api.Response{}, errors.New("mockProvider: no more responses")
	}
	m.calls = append(m.calls, messages)
	resp := m.responses[m.idx]
	m.idx++
	// emit text blocks via onChunk (simulates streaming)
	for _, b := range resp.Content {
		if b.Type == api.BlockText {
			onChunk(b.Text)
		}
	}
	return resp, nil
}

func (m *mockProvider) Model() string       { return "mock" }
func (m *mockProvider) SetModel(_ string)   {}

// helpers

func textResp(text string) api.Response {
	return api.Response{
		Content:    []api.Block{{Type: api.BlockText, Text: text}},
		StopReason: api.StopEndTurn,
	}
}

func toolResp(id, name, input string) api.Response {
	return api.Response{
		Content:    []api.Block{{Type: api.BlockToolUse, ToolUseID: id, ToolName: name, ToolInput: input}},
		StopReason: api.StopToolUse,
	}
}

// tests

func TestRun_TextResponse(t *testing.T) {
	mock := &mockProvider{responses: []api.Response{textResp("hello")}}

	var texts []string
	_, err := agent.Run(context.Background(), mock, nil, nil, agent.Hooks{
		OnText: func(text string) { texts = append(texts, text) },
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(texts) != 1 || texts[0] != "hello" {
		t.Fatalf("expected [hello], got %v", texts)
	}
}

func TestRun_ToolUse_SendsResult(t *testing.T) {
	mock := &mockProvider{responses: []api.Response{
		toolResp("id1", "bash", `{"command":"ls"}`),
		textResp("done"),
	}}

	var toolCalls []string
	_, err := agent.Run(context.Background(), mock, nil, nil, agent.Hooks{
		OnToolUse: func(name, input string) agent.ToolResult {
			toolCalls = append(toolCalls, name)
			return agent.ToolResult{Content: "file.txt", IsError: false}
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(toolCalls) != 1 || toolCalls[0] != "bash" {
		t.Fatalf("expected [bash], got %v", toolCalls)
	}

	// second Send should have the tool result in history
	secondCall := mock.calls[1]
	last := secondCall[len(secondCall)-1]
	if last.Role != api.RoleUser || last.Content[0].Type != api.BlockToolResult {
		t.Fatal("expected tool result in second Send")
	}
	if last.Content[0].ToolResult != "file.txt" {
		t.Fatalf("expected 'file.txt', got %q", last.Content[0].ToolResult)
	}
}

func TestRun_ToolUse_DeniedIsError(t *testing.T) {
	mock := &mockProvider{responses: []api.Response{
		toolResp("id1", "bash", `{}`),
		textResp("ok"),
	}}

	_, err := agent.Run(context.Background(), mock, nil, nil, agent.Hooks{
		OnToolUse: func(name, input string) agent.ToolResult {
			return agent.ToolResult{Content: "user denied", IsError: true}
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	secondCall := mock.calls[1]
	last := secondCall[len(secondCall)-1]
	if !last.Content[0].IsError {
		t.Fatal("denied tool must have IsError=true")
	}
}

func TestRun_MultipleToolsInOneTurn(t *testing.T) {
	mock := &mockProvider{responses: []api.Response{
		{
			Content: []api.Block{
				{Type: api.BlockToolUse, ToolUseID: "1", ToolName: "bash", ToolInput: `{}`},
				{Type: api.BlockToolUse, ToolUseID: "2", ToolName: "read_file", ToolInput: `{}`},
			},
			StopReason: api.StopToolUse,
		},
		textResp("done"),
	}}

	var called []string
	_, err := agent.Run(context.Background(), mock, nil, nil, agent.Hooks{
		OnToolUse: func(name, input string) agent.ToolResult {
			called = append(called, name)
			return agent.ToolResult{Content: "ok"}
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(called) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(called))
	}
}

func TestRun_NoToolHandler_ReturnsError(t *testing.T) {
	mock := &mockProvider{responses: []api.Response{
		toolResp("id1", "bash", `{}`),
		textResp("done"),
	}}

	_, err := agent.Run(context.Background(), mock, nil, nil, agent.Hooks{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	secondCall := mock.calls[1]
	last := secondCall[len(secondCall)-1]
	if !last.Content[0].IsError {
		t.Fatal("nil OnToolUse must return IsError=true")
	}
}

func TestRun_HistoryGrowsCorrectly(t *testing.T) {
	initial := []api.Message{
		{Role: api.RoleUser, Content: []api.Block{{Type: api.BlockText, Text: "hello"}}},
	}
	mock := &mockProvider{responses: []api.Response{textResp("hi")}}

	history, err := agent.Run(context.Background(), mock, initial, nil, agent.Hooks{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// initial user + assistant response
	if len(history) != 2 {
		t.Fatalf("expected history len 2, got %d", len(history))
	}
	if history[1].Role != api.RoleAssistant {
		t.Fatal("second message must be assistant")
	}
}
