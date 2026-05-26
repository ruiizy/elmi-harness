// Package openai implements provider.Provider using the OpenAI-compatible API.
// Point baseURL at http://localhost:11434/v1 for Ollama, http://localhost:1234/v1
// for LM Studio, or any other OpenAI-compatible local server.
package openai

import (
	"context"
	"os"
	"strings"
	"sync"

	sdk "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/ruiizy/elmi-harness/internal/api"
)

type Provider struct {
	client    sdk.Client
	model     string
	maxTokens int64
	system    string

	mu    sync.Mutex
	total api.Usage
}

func New(model, system string, maxTokens int64, baseURL string) *Provider {
	opts := []option.RequestOption{}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
		if os.Getenv("OPENAI_API_KEY") == "" {
			opts = append(opts, option.WithAPIKey("local"))
		}
	}
	return &Provider{
		client:    sdk.NewClient(opts...),
		model:     model,
		maxTokens: maxTokens,
		system:    system,
	}
}

func (p *Provider) Model() string { return p.model }

func (p *Provider) SetModel(name string) {
	p.mu.Lock()
	p.model = name
	p.mu.Unlock()
}

func (p *Provider) TotalUsage() api.Usage {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.total
}

func (p *Provider) EstimatedCostUSD() float64 {
	p.mu.Lock()
	u := p.total
	model := p.model
	p.mu.Unlock()

	rates, ok := modelPricing[model]
	if !ok {
		return -1
	}
	return float64(u.InputTokens)*rates.input/1_000_000 +
		float64(u.OutputTokens)*rates.output/1_000_000
}

type pricing struct{ input, output float64 }

var modelPricing = map[string]pricing{
	"gpt-4o":      {2.50, 10.00},
	"gpt-4o-mini": {0.15, 0.60},
	"gpt-4-turbo": {10.00, 30.00},
	"o1":          {15.00, 60.00},
	"o1-mini":     {1.10, 4.40},
}

func (p *Provider) Stream(ctx context.Context, messages []api.Message, tools []api.ToolDef, onChunk func(string)) (api.Response, error) {
	p.mu.Lock()
	model := p.model
	maxTokens := p.maxTokens
	system := p.system
	p.mu.Unlock()

	stream := p.client.Chat.Completions.NewStreaming(ctx, sdk.ChatCompletionNewParams{
		Model:     sdk.ChatModel(model),
		MaxTokens: sdk.Int(maxTokens),
		Messages:  toMessages(system, messages),
		Tools:     toTools(tools),
	})

	var acc sdk.ChatCompletionAccumulator
	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				onChunk(choice.Delta.Content)
			}
		}
	}
	if err := stream.Err(); err != nil {
		return api.Response{}, err
	}

	if len(acc.Choices) == 0 {
		return api.Response{StopReason: api.StopOther}, nil
	}

	choice := acc.Choices[0]
	out := api.Response{StopReason: fromFinishReason(choice.FinishReason)}

	if choice.Message.Content != "" {
		out.Content = append(out.Content, api.Block{Type: api.BlockText, Text: choice.Message.Content})
	}
	for _, tc := range choice.Message.ToolCalls {
		out.Content = append(out.Content, api.Block{
			Type:      api.BlockToolUse,
			ToolUseID: tc.ID,
			ToolName:  tc.Function.Name,
			ToolInput: tc.Function.Arguments,
		})
	}

	out.Usage = api.Usage{
		InputTokens:  int(acc.Usage.PromptTokens),
		OutputTokens: int(acc.Usage.CompletionTokens),
	}
	p.mu.Lock()
	p.total = p.total.Add(out.Usage)
	p.mu.Unlock()

	return out, nil
}

func toMessages(system string, messages []api.Message) []sdk.ChatCompletionMessageParamUnion {
	out := make([]sdk.ChatCompletionMessageParamUnion, 0, len(messages)+1)
	if system != "" {
		out = append(out, sdk.SystemMessage(system))
	}
	for _, m := range messages {
		switch m.Role {
		case api.RoleUser:
			var sb strings.Builder
			for _, b := range m.Content {
				switch b.Type {
				case api.BlockText:
					if sb.Len() > 0 {
						sb.WriteByte('\n')
					}
					sb.WriteString(b.Text)
				case api.BlockToolResult:
					out = append(out, sdk.ToolMessage(b.ToolResult, b.ToolUseID))
				}
			}
			if sb.Len() > 0 {
				out = append(out, sdk.UserMessage(sb.String()))
			}
		case api.RoleAssistant:
			var asst sdk.ChatCompletionAssistantMessageParam
			var sb strings.Builder
			for _, b := range m.Content {
				switch b.Type {
				case api.BlockText:
					if sb.Len() > 0 {
						sb.WriteByte('\n')
					}
					sb.WriteString(b.Text)
				case api.BlockToolUse:
					asst.ToolCalls = append(asst.ToolCalls, sdk.ChatCompletionMessageToolCallParam{
						ID: b.ToolUseID,
						Function: sdk.ChatCompletionMessageToolCallFunctionParam{
							Name:      b.ToolName,
							Arguments: b.ToolInput,
						},
					})
				}
			}
			if sb.Len() > 0 {
				asst.Content.OfString = sdk.String(sb.String())
			}
			out = append(out, sdk.ChatCompletionMessageParamUnion{OfAssistant: &asst})
		}
	}
	return out
}

func toTools(tools []api.ToolDef) []sdk.ChatCompletionToolParam {
	out := make([]sdk.ChatCompletionToolParam, 0, len(tools))
	for _, t := range tools {
		out = append(out, sdk.ChatCompletionToolParam{
			Function: sdk.FunctionDefinitionParam{
				Name:        t.Name,
				Description: sdk.String(t.Description),
				Parameters: sdk.FunctionParameters{
					"type":       "object",
					"properties": t.InputSchema,
					"required":   t.Required,
				},
			},
		})
	}
	return out
}

func fromFinishReason(reason string) api.StopReason {
	switch reason {
	case "tool_calls":
		return api.StopToolUse
	case "stop":
		return api.StopEndTurn
	default:
		return api.StopOther
	}
}
