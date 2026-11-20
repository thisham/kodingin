package conversation

import "testing"

func TestRetainRecent(t *testing.T) {
	for _, tt := range []struct {
		name  string
		limit int
		want  []Role
	}{
		{"keeps complete recent turns", 3, []Role{RoleSystem, RoleUser}},
		{"does not split tool turn", 2, []Role{RoleSystem, RoleUser}},
		{"zero is ignored", 0, []Role{RoleSystem, RoleUser, RoleAssistant, RoleTool, RoleUser}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := New("sys")
			c.AddUser("old")
			c.AddAssistant(Message{Content: "answer", ToolCalls: []ToolCall{{ID: "call"}}})
			c.AddToolResult("call", "result")
			c.AddUser("new")
			c.RetainRecent(tt.limit)
			got := c.Messages()
			roles := make([]Role, 0, len(got))
			for _, m := range got {
				roles = append(roles, m.Role)
			}
			if len(roles) != len(tt.want) {
				t.Fatalf("roles = %v, want %v", roles, tt.want)
			}
			for i := range roles {
				if roles[i] != tt.want[i] {
					t.Fatalf("roles = %v, want %v", roles, tt.want)
				}
			}
		})
	}
}
