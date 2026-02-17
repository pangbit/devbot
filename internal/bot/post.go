package bot

import "strings"

// PostElement represents a single element in a post paragraph.
type PostElement struct {
	Tag   string   `json:"tag"`
	Text  string   `json:"text"`
	Href  string   `json:"href,omitempty"`
	Style []string `json:"style,omitempty"`
}

// Post represents a Feishu post (rich text) message.
type Post struct {
	Title      string
	Paragraphs [][]PostElement
}

func TextElement(text string) PostElement {
	return PostElement{Tag: "text", Text: text}
}

func BoldElement(text string) PostElement {
	return PostElement{Tag: "text", Text: text, Style: []string{"bold"}}
}

func LinkElement(text, href string) PostElement {
	return PostElement{Tag: "a", Text: text, Href: href}
}

// PlainText renders the post as plain text (for fallback and test spies).
func (p Post) PlainText() string {
	var sb strings.Builder
	if p.Title != "" {
		sb.WriteString(p.Title)
		sb.WriteString("\n\n")
	}
	for i, para := range p.Paragraphs {
		for _, elem := range para {
			sb.WriteString(elem.Text)
		}
		if i < len(p.Paragraphs)-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
