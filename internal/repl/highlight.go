package repl

import (
	"strings"
	"unicode"
)

const (
	ansiReset   = "\033[0m"
	ansiBoldCyan = "\033[1;36m"
	ansiYellow  = "\033[33m"
	ansiGreen   = "\033[32m"
	ansiMagenta = "\033[35m"
	ansiRed     = "\033[31m"
)

var keywordSet map[string]bool
var functionSet map[string]bool

func init() {
	keywordSet = make(map[string]bool, len(allKeywords))
	functionSet = make(map[string]bool, len(allFunctions))
}

func initHighlightSets() {
	if len(keywordSet) > 0 {
		return
	}
	for _, k := range allKeywords {
		keywordSet[k] = true
	}
	for _, f := range allFunctions {
		functionSet[f] = true
	}
}

// Highlight applies ANSI color codes to an ES|QL query string.
func Highlight(query string) string {
	initHighlightSets()

	var out strings.Builder
	out.Grow(len(query) * 2)
	i := 0

	for i < len(query) {
		ch := query[i]

		// Quoted strings (single or double)
		if ch == '\'' || ch == '"' {
			out.WriteString(ansiGreen)
			out.WriteByte(ch)
			i++
			for i < len(query) {
				out.WriteByte(query[i])
				if query[i] == ch {
					i++
					break
				}
				i++
			}
			out.WriteString(ansiReset)
			continue
		}

		// Numbers
		if ch >= '0' && ch <= '9' {
			out.WriteString(ansiMagenta)
			for i < len(query) && (query[i] >= '0' && query[i] <= '9' || query[i] == '.') {
				out.WriteByte(query[i])
				i++
			}
			out.WriteString(ansiReset)
			continue
		}

		// Multi-char operators
		if i+1 < len(query) {
			two := query[i : i+2]
			if two == "==" || two == "!=" || two == ">=" || two == "<=" || two == "::" {
				out.WriteString(ansiRed)
				out.WriteString(two)
				out.WriteString(ansiReset)
				i += 2
				continue
			}
		}

		// Single-char operators
		if ch == '|' || ch == '<' || ch == '>' || ch == '=' || ch == '+' || ch == '-' || ch == '*' || ch == '/' || ch == '%' || ch == ':' {
			out.WriteString(ansiRed)
			out.WriteByte(ch)
			out.WriteString(ansiReset)
			i++
			continue
		}

		// Words (identifiers, keywords, functions)
		if ch == '_' || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
			start := i
			for i < len(query) && (query[i] == '_' || query[i] == '.' || unicode.IsLetter(rune(query[i])) || unicode.IsDigit(rune(query[i]))) {
				i++
			}
			word := query[start:i]
			upper := strings.ToUpper(word)

			if keywordSet[upper] {
				out.WriteString(ansiBoldCyan)
				out.WriteString(word)
				out.WriteString(ansiReset)
			} else if functionSet[upper] {
				out.WriteString(ansiYellow)
				out.WriteString(word)
				out.WriteString(ansiReset)
			} else {
				out.WriteString(word)
			}
			continue
		}

		// Everything else (whitespace, parens, commas, etc.)
		out.WriteByte(ch)
		i++
	}

	return out.String()
}
