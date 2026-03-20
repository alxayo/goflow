// tools.go defines the read_memory and write_memory tool specifications
// that can be registered with SDK sessions so agents can interact with
// shared memory during execution.
package memory

// ToolSpec describes a custom tool that can be registered with an SDK session.
type ToolSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ReadMemoryTool returns the tool specification for the read_memory tool.
func ReadMemoryTool() ToolSpec {
	return ToolSpec{
		Name:        "read_memory",
		Description: "Read the shared memory file containing context and findings from other agents running in parallel. Returns the full contents of the shared memory.",
	}
}

// WriteMemoryTool returns the tool specification for the write_memory tool.
func WriteMemoryTool() ToolSpec {
	return ToolSpec{
		Name:        "write_memory",
		Description: "Append an entry to the shared memory file. Other agents running in parallel can read this. Use this to share important findings, context, or signals with other agents. The entry will be timestamped and attributed to your agent name.",
	}
}

// MemoryPromptAddendum returns the text to append to an agent's system prompt
// when shared memory tools are available (but inject_into_prompt is false).
func MemoryPromptAddendum() string {
	return `

You have access to a shared memory file via the read_memory and write_memory tools.
- Use read_memory to check for context from other agents running in parallel.
- Use write_memory to record findings that other agents should know about.
- Check shared memory periodically to stay aware of other agents' discoveries.`
}
