package markdown

import "strings"

func ReplaceManagedBlock(body, startMarker, endMarker, generated string) string {
	start := strings.Index(body, startMarker)
	end := strings.Index(body, endMarker)
	block := startMarker + "\n" + generated + "\n" + endMarker

	if start >= 0 && end > start {
		end += len(endMarker)
		return body[:start] + block + body[end:]
	}

	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return block + "\n"
	}
	if strings.HasSuffix(body, "\n") {
		return body + "\n" + block + "\n"
	}
	return body + "\n\n" + block + "\n"
}
