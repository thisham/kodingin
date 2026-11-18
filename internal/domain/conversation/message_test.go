package conversation

import "testing"

func TestRestore(t *testing.T) {
	call := ToolCall{ID: "c1", Name: "read_file"}
	for _, tt := range []struct {
		name string
		in   []Message
		want int
	}{
		{"plain", []Message{{Role: RoleUser, Content: "hi"}, {Role: RoleAssistant, Content: "yo"}}, 2},
		{"drops saved system", []Message{{Role: RoleSystem, Content: "old"}, {Role: RoleUser, Content: "hi"}}, 1},
		{"complete tool turn kept", []Message{{Role: RoleUser}, {Role: RoleAssistant, ToolCalls: []ToolCall{call}}, {Role: RoleTool, ToolCallID: "c1"}}, 3},
		{"dangling call trimmed", []Message{{Role: RoleUser}, {Role: RoleAssistant, ToolCalls: []ToolCall{call}}}, 1},
		{"partial results trimmed", []Message{{Role: RoleUser}, {Role: RoleAssistant, ToolCalls: []ToolCall{call, {ID: "c2"}}}, {Role: RoleTool, ToolCallID: "c1"}}, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := New("sys")
			c.Restore(tt.in)
			if c.Len() != tt.want || c.Messages()[0].Content != "sys" {
				t.Fatalf("len = %d, msgs = %+v", c.Len(), c.Messages())
			}
		})
	}
}

func TestCompact(t *testing.T) {
	c := New("sys")
	c.AddUser("a")
	c.AddAssistant(Message{Content: "b"})
	c.Compact("did a and b")
	m := c.Messages()
	if c.Len() != 1 || m[1].Role != RoleUser || m[1].Content != CompactPrefix+"did a and b" {
		t.Fatalf("%+v", m)
	}
}
