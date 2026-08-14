package asset

import "strings"

type browseQuery struct {
	Text   string
	Author string
	Tags   []string
}

func parseBrowseQuery(raw string) browseQuery {
	var parsed browseQuery
	var text []string
	for _, token := range browseTokens(raw) {
		lower := normalizeBrowseText(token)
		switch {
		case strings.HasPrefix(lower, "tag:") && len(lower) > len("tag:"):
			parsed.Tags = append(parsed.Tags, normalizeBrowseText(token[len("tag:"):]))
		case strings.HasPrefix(lower, "author:") && len(lower) > len("author:") && parsed.Author == "":
			parsed.Author = normalizeBrowseText(token[len("author:"):])
		default:
			text = append(text, lower)
		}
	}
	parsed.Text = strings.Join(text, " ")
	return parsed
}

func browseTokens(raw string) []string {
	var tokens []string
	var token strings.Builder
	quoted := false
	for _, char := range raw {
		switch {
		case char == '"':
			quoted = !quoted
		case !quoted && (char == ' ' || char == '\t' || char == '\n' || char == '\r'):
			if token.Len() > 0 {
				tokens = append(tokens, token.String())
				token.Reset()
			}
		default:
			token.WriteRune(char)
		}
	}
	if token.Len() > 0 {
		tokens = append(tokens, token.String())
	}
	return tokens
}

func normalizeBrowseText(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
