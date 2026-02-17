package bot

// CardMsg represents a Feishu interactive card message with markdown content.
type CardMsg struct {
	Title    string
	Content  string // Markdown formatted content
	Template string // Header color: blue/green/red/purple (defaults to blue)
}
