package memory

import (
	"strings"
	"testing"
)

func TestReadMemoryTool(t *testing.T) {
	tool := ReadMemoryTool()

	if tool.Name != "read_memory" {
		t.Errorf("Name = %q, want %q", tool.Name, "read_memory")
	}
	if tool.Description == "" {
		t.Error("Description is empty, want non-empty")
	}
}

func TestWriteMemoryTool(t *testing.T) {
	tool := WriteMemoryTool()

	if tool.Name != "write_memory" {
		t.Errorf("Name = %q, want %q", tool.Name, "write_memory")
	}
	if tool.Description == "" {
		t.Error("Description is empty, want non-empty")
	}
}

func TestMemoryPromptAddendum(t *testing.T) {
	addendum := MemoryPromptAddendum()

	if addendum == "" {
		t.Fatal("MemoryPromptAddendum() is empty, want non-empty")
	}
	if !strings.Contains(addendum, "read_memory") {
		t.Errorf("addendum = %q, want it to mention read_memory", addendum)
	}
	if !strings.Contains(addendum, "write_memory") {
		t.Errorf("addendum = %q, want it to mention write_memory", addendum)
	}
}
