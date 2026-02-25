package markdown

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const separator = "---\n"

func SplitFrontmatter(content string) (map[string]any, string, error) {
	if !strings.HasPrefix(content, separator) {
		return map[string]any{}, content, nil
	}
	rest := strings.TrimPrefix(content, separator)
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return nil, "", fmt.Errorf("invalid frontmatter: missing closing separator")
	}
	raw := rest[:idx]
	body := rest[idx+len("\n---\n"):]

	decoded := map[string]any{}
	if err := yaml.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, "", fmt.Errorf("unmarshal frontmatter: %w", err)
	}
	return decoded, body, nil
}

func RenderFrontmatter(meta map[string]any, body string) (string, error) {
	raw, err := yaml.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("marshal frontmatter: %w", err)
	}
	buf := bytes.Buffer{}
	buf.WriteString(separator)
	buf.Write(raw)
	buf.WriteString(separator)
	if !strings.HasPrefix(body, "\n") {
		buf.WriteString("\n")
	}
	buf.WriteString(body)
	return buf.String(), nil
}
