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
func (c *Conversation) Reset() { c.messages = c.messages[:1] }
