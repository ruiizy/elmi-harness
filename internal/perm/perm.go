package perm

import "fmt"

type Mode int

const (
	ModeAlwaysAsk   Mode = iota // ask on every call (default)
	ModeAlwaysAllow             // never ask
	ModeAllowList               // auto-run named tools, ask rest
	ModeAskOnce                 // ask first call per tool, auto after
)

func (m Mode) String() string {
	switch m {
	case ModeAlwaysAllow:
		return "always-allow"
	case ModeAllowList:
		return "allow-list"
	case ModeAskOnce:
		return "ask-once"
	default:
		return "always-ask"
	}
}

// Result is the outcome of a user confirmation prompt.
type Result int

const (
	ResultYes    Result = iota // allow once
	ResultNo                   // deny
	ResultAlways               // allow + grant session-level permission
	ResultOther                // user typed a custom message for the model
)

// ConfirmFunc is the UI callback injected into Check.
// Returns (result, customMessage). customMessage is only used when result == ResultOther.
type ConfirmFunc func(name, input string) (Result, string)

// Config holds permission state for one session.
type Config struct {
	Mode      Mode
	AllowList map[string]bool

	sessionGrant map[string]bool // granted via ResultAlways
	askOnceGrant map[string]bool // ModeAskOnce: tools already answered yes
}

func New() *Config {
	return &Config{
		Mode:         ModeAlwaysAsk,
		AllowList:    map[string]bool{},
		sessionGrant: map[string]bool{},
		askOnceGrant: map[string]bool{},
	}
}

func (c *Config) SetMode(m Mode) { c.Mode = m }

func (c *Config) SetAllowList(tools []string) {
	c.AllowList = make(map[string]bool, len(tools))
	for _, t := range tools {
		c.AllowList[t] = true
	}
}

func (c *Config) ResetAskOnce() { c.askOnceGrant = map[string]bool{} }

// Check decides whether a tool should run.
// confirm is called only when a prompt is actually needed.
// Returns (run, override): run=true → execute tool;
// run=false, override="" → denied; run=false, override≠"" → send override to model.
func (c *Config) Check(name, input string, confirm ConfirmFunc) (run bool, override string) {
	if c.sessionGrant[name] {
		fmt.Printf("[auto] %s (session grant)\n", name)
		return true, ""
	}

	switch c.Mode {
	case ModeAlwaysAllow:
		return true, ""
	case ModeAllowList:
		if c.AllowList[name] {
			fmt.Printf("[auto] %s (allow-list)\n", name)
			return true, ""
		}
	case ModeAskOnce:
		if c.askOnceGrant[name] {
			fmt.Printf("[auto] %s (ask-once)\n", name)
			return true, ""
		}
	}

	result, msg := confirm(name, input)
	switch result {
	case ResultAlways:
		c.sessionGrant[name] = true
		return true, ""
	case ResultYes:
		if c.Mode == ModeAskOnce {
			c.askOnceGrant[name] = true
		}
		return true, ""
	case ResultOther:
		return false, msg
	default: // ResultNo
		return false, ""
	}
}
