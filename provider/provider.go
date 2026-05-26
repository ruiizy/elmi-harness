// Package provider defines the LLM-backend interface. The harness only
// talks to providers through Provider — swap implementations to swap models or SDKs.
package provider

import (
	"context"

	"github.com/ruiizy/elmi-harness/internal/api"
)

type Provider interface {
	// Stream sends messages and delivers text chunks via onChunk as they arrive.
	// Returns a Response with tool use blocks and metadata after the stream ends.
	// Text is included in Response.Content for history but delivered exclusively
	// via onChunk — callers must not print it again.
	Stream(ctx context.Context, messages []api.Message, tools []api.ToolDef, onChunk func(string)) (api.Response, error)
	Model() string
	SetModel(name string)
}
