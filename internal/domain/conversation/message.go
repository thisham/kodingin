// Package conversation is the core domain: a coding session is a Conversation of Messages.
package conversation

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall is the model asking to run a tool.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string // raw JSON
}

// Message is a value object.
type Message struct {
	Role       Role
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string // set when Role == RoleTool
}

// Conversation is the aggregate root. All mutation goes through its methods.
type Conversation struct {
	messages []Message
}

func New(systemPrompt string) *Conversation {
	return &Conversation{messages: []Message{{Role: RoleSystem, Content: systemPrompt}}}
}

func (c *Conversation) AddUser(text string) {
	c.messages = append(c.messages, Message{Role: RoleUser, Content: text})
}
func (c *Conversation) AddAssistant(m Message) {
	m.Role = RoleAssistant
	c.messages = append(c.messages, m)
}
func (c *Conversation) AddToolResult(callID, out string) {
	c.messages = append(c.messages, Message{Role: RoleTool, ToolCallID: callID, Content: out})
}

// Messages returns a copy so callers cannot break invariants.
func (c *Conversation) Messages() []Message {
	out := make([]Message, len(c.messages))
	copy(out, c.messages)
	return out
}

// Reset keeps only the system prompt.
func (c *Conversation) Reset() {
	c.messages = []Message{c.messages[0]}
}

// Restore replaces the history with msgs, keeping this conversation's own system prompt.
// Saved system messages are dropped and a dangling tool call (an interrupted turn) is trimmed,
// so the result is always a valid request for the model.
func (c *Conversation) Restore(msgs []Message) {
	c.Reset()
	for _, m := range msgs {
		if m.Role != RoleSystem {
			c.messages = append(c.messages, m)
		}
	}
	c.trimDangling()
}

// Compact replaces the history with a summary of it.
func (c *Conversation) Compact(summary string) {
	c.messages = []Message{c.messages[0], {Role: RoleUser, Content: CompactPrefix + summary}}
}

// CompactPrefix marks a message that stands in for compacted history.
const CompactPrefix = "[Summary of the earlier conversation, compacted to save context]\n\n"

// Len counts messages excluding the system prompt.
func (c *Conversation) Len() int { return len(c.messages) - 1 }

// trimDangling drops a trailing assistant tool call whose results never arrived.
func (c *Conversation) trimDangling() {
	for {
		last := len(c.messages) - 1
		if last < 1 {
			return
		}
		// Walk back over tool results to the assistant message that requested them.
		i := last
		for i > 0 && c.messages[i].Role == RoleTool {
			i--
		}
		a := c.messages[i]
		if a.Role != RoleAssistant || len(a.ToolCalls) == 0 {
			return
		}
		answered := map[string]bool{}
		for _, m := range c.messages[i+1:] {
			answered[m.ToolCallID] = true
		}
		complete := true
		for _, tc := range a.ToolCalls {
			if !answered[tc.ID] {
				complete = false
			}
		}
		if complete {
			return
		}
		clear(c.messages[i:])
		c.messages = c.messages[:i]
	}
}
