// Package anthropic is the Anthropic SDK implementation of provider.Provider.
// It is the only file in the harness that imports the Anthropic SDK.
package anthropic

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/ruiizy/elmi-harness/internal/api"
)

type Provider struct {
	client    sdk.Client
	model     sdk.Model
	maxTokens int64
	system    string

	mu    sync.Mutex
	total api.Usage
}

func New(model string, maxTokens int64, system string) *Provider {
	return &Provider{
		client:    sdk.NewClient(),
		model:     sdk.Model(model),
		maxTokens: maxTokens,
		system:    system,
	}
}

func (p *Provider) Model() string { return string(p.model) }

func (p *Provider) SetModel(name string) {
	p.mu.Lock()
	p.model = sdk.Model(name)
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
	model := string(p.model)
	p.mu.Unlock()

	rates, ok := modelPricing[model]
	if !ok {
		return -1
	}
	return float64(u.InputTokens)*rates.input/1_000_000 +
		float64(u.OutputTokens)*rates.output/1_000_000 +
		float64(u.CacheCreationTokens)*rates.cacheWrite/1_000_000 +
		float64(u.CacheReadTokens)*rates.cacheRead/1_000_000
}

type pricing struct{ input, output, cacheWrite, cacheRead float64 }

var modelPricing = map[string]pricing{
	"claude-opus-4-7-20250514":  {15.00, 75.00, 18.75, 1.50},
	"claude-opus-4-6":           {15.00, 75.00, 18.75, 1.50},
	"claude-sonnet-4-6":         {3.00, 15.00, 3.75, 0.30},
	"claude-haiku-4-5-20251001": {1.00, 5.00, 1.25, 0.10},
}

func (p *Provider) Stream(ctx context.Context, messages []api.Message, tools []api.ToolDef, onChunk func(string)) (api.Response, error) {
	p.mu.Lock()
	model := p.model
	maxTokens := p.maxTokens
	system := p.system
	p.mu.Unlock()

	stream := p.client.Messages.NewStreaming(ctx, sdk.MessageNewParams{
		Model:     model,
		MaxTokens: maxTokens,
		System:    []sdk.TextBlockParam{{Text: system}},
		Messages:  p.toMessages(messages),
		Tools:     p.toTools(tools),
		Thinking: sdk.ThinkingConfigParamUnion{
			OfAdaptive: &sdk.ThinkingConfigAdaptiveParam{},
		},
	})

	var acc sdk.Message
	var text strings.Builder

	for stream.Next() {
		event := stream.Current()
		if err := acc.Accumulate(event); err != nil {
			return api.Response{}, err
		}
		if e, ok := event.AsAny().(sdk.ContentBlockDeltaEvent); ok {
			if d, ok := e.Delta.AsAny().(sdk.TextDelta); ok {
				onChunk(d.Text)
				text.WriteString(d.Text)
			}
		}
	}
	if err := stream.Err(); err != nil {
		return api.Response{}, err
	}

	out := api.Response{StopReason: fromStopReason(acc.StopReason)}

	if text.Len() > 0 {
		out.Content = append(out.Content, api.Block{Type: api.BlockText, Text: text.String()})
	}
	for _, block := range acc.Content {
		switch v := block.AsAny().(type) {
		case sdk.ToolUseBlock:
			out.Content = append(out.Content, api.Block{
				Type:      api.BlockToolUse,
				ToolUseID: v.ID,
				ToolName:  v.Name,
				ToolInput: v.JSON.Input.Raw(),
			})
		}
	}

	out.Usage = api.Usage{
		InputTokens:         int(acc.Usage.InputTokens),
		OutputTokens:        int(acc.Usage.OutputTokens),
		CacheCreationTokens: int(acc.Usage.CacheCreationInputTokens),
		CacheReadTokens:     int(acc.Usage.CacheReadInputTokens),
	}
	p.mu.Lock()
	p.total = p.total.Add(out.Usage)
	p.mu.Unlock()

	return out, nil
}

func (p *Provider) toMessages(messages []api.Message) []sdk.MessageParam {
	out := make([]sdk.MessageParam, 0, len(messages))
	for _, m := range messages {
		blocks := make([]sdk.ContentBlockParamUnion, 0, len(m.Content))
		for _, b := range m.Content {
			switch b.Type {
			case api.BlockText:
				blocks = append(blocks, sdk.NewTextBlock(b.Text))
			case api.BlockToolUse:
				blocks = append(blocks, sdk.ContentBlockParamUnion{
					OfToolUse: &sdk.ToolUseBlockParam{
						ID:    b.ToolUseID,
						Name:  b.ToolName,
						Input: json.RawMessage(b.ToolInput),
					},
				})
			case api.BlockToolResult:
				blocks = append(blocks, sdk.NewToolResultBlock(b.ToolUseID, b.ToolResult, b.IsError))
			}
		}
		switch m.Role {
		case api.RoleUser:
			out = append(out, sdk.NewUserMessage(blocks...))
		case api.RoleAssistant:
			out = append(out, sdk.NewAssistantMessage(blocks...))
		}
	}
	return out
}

func (p *Provider) toTools(tools []api.ToolDef) []sdk.ToolUnionParam {
	out := make([]sdk.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		out = append(out, sdk.ToolUnionParam{
			OfTool: &sdk.ToolParam{
				Name:        t.Name,
				Description: sdk.String(t.Description),
				InputSchema: sdk.ToolInputSchemaParam{
					Properties: t.InputSchema,
					Required:   t.Required,
				},
			},
		})
	}
	return out
}

func fromStopReason(s sdk.StopReason) api.StopReason {
	switch s {
	case sdk.StopReasonEndTurn:
		return api.StopEndTurn
	case sdk.StopReasonToolUse:
		return api.StopToolUse
	default:
		return api.StopOther
	}
}
