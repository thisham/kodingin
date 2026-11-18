package conversation

import (
	"reflect"
	"testing"
)

func TestDiscardedHistoryReleased(t *testing.T) {
	for _, tt := range []struct {
		name    string
		discard func(*Conversation)
		want    []Message
	}{
		{"reset", (*Conversation).Reset, []Message{{Role: RoleSystem, Content: "sys"}}},
		{"compact", func(c *Conversation) { c.Compact("summary") }, []Message{{Role: RoleSystem, Content: "sys"}, {Role: RoleUser, Content: CompactPrefix + "summary"}}},
		{"restore shorter", func(c *Conversation) { c.Restore([]Message{{Role: RoleUser, Content: "new"}}) }, []Message{{Role: RoleSystem, Content: "sys"}, {Role: RoleUser, Content: "new"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := New("sys")
			for range 32 {
				c.AddAssistant(Message{Content: "old", ToolCalls: []ToolCall{{ID: "old", Arguments: "large arguments"}}})
			}
			old := &c.messages[0]
			tt.discard(c)
			if !reflect.DeepEqual(c.Messages(), tt.want) {
				t.Fatalf("messages = %+v", c.Messages())
			}
			if &c.messages[0] == old {
				t.Fatal("discarded backing array is still retained")
			}
			for _, m := range c.messages[len(c.messages):cap(c.messages)] {
				if !reflect.DeepEqual(m, Message{}) {
					t.Fatalf("stale message retained: %+v", m)
				}
			}
		})
	}
}

func TestTrimDanglingClearsReferences(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(map[bool]string{false: "unanswered", true: "partial results"}[partial], func(t *testing.T) {
			c := New("sys")
			c.AddUser("keep")
			c.AddAssistant(Message{Content: "discard", ToolCalls: []ToolCall{{ID: "one", Arguments: "discard"}, {ID: "two"}}})
			if partial {
				c.AddToolResult("one", "discard result")
			}
			c.trimDangling()
			if c.Len() != 1 || c.messages[1].Content != "keep" {
				t.Fatalf("messages = %+v", c.messages)
			}
			for _, m := range c.messages[len(c.messages):cap(c.messages)] {
				if !reflect.DeepEqual(m, Message{}) {
					t.Fatalf("stale message retained: %+v", m)
				}
			}
		})
	}
}
