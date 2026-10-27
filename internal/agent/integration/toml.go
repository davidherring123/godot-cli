package integration

import (
	"fmt"
	"strconv"
	"strings"
)

type tomlBlock struct {
	start int
	end   int
	path  string
	array bool
}

type tomlAssignment struct {
	key   string
	value string
	start int
	end   int
}

func splitTOMLines(data []byte) ([]string, string) {
	newline := "\n"
	text := string(data)

	if strings.Contains(text, "\r\n") {
		newline = "\r\n"
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}

	return strings.Split(text, "\n"), newline
}

func parseTOMLBlocks(lines []string) []tomlBlock {
	starts := []int{0}
	paths := []string{""}
	arrays := []bool{false}

	for index, line := range lines {
		path, array, ok := parseTOMLHeader(line)
		if !ok {
			continue
		}

		starts = append(starts, index)
		paths = append(paths, path)
		arrays = append(arrays, array)
	}

	blocks := make([]tomlBlock, 0, len(starts))

	for index, start := range starts {
		end := len(lines)

		if index+1 < len(starts) {
			end = starts[index+1]
		}

		blocks = append(blocks, tomlBlock{
			start: start,
			end:   end,
			path:  paths[index],
			array: arrays[index],
		})
	}

	return blocks
}

func parseTOMLHeader(line string) (string, bool, bool) {
	trimmed := strings.TrimSpace(line)

	if !strings.HasPrefix(trimmed, "[") {
		return "", false, false
	}

	array := strings.HasPrefix(trimmed, "[[")

	contentStart := 1
	if array {
		contentStart = 2
	}

	var quote byte
	escaped := false

	for index := contentStart; index < len(trimmed); index++ {
		character := trimmed[index]

		if quote != 0 {
			if quote == '"' {
				if escaped {
					escaped = false
				} else if character == '\\' {
					escaped = true
				} else if character == quote {
					quote = 0
				}
			} else if character == quote {
				quote = 0
			}

			continue
		}

		switch character {
		case '"', '\'':
			quote = character
		case ']':
			if array {
				if index+1 < len(trimmed) && trimmed[index+1] == ']' {
					return normalizeTOMLKeyPath(trimmed[contentStart:index]), true, true
				}

				return "", false, false
			}

			return normalizeTOMLKeyPath(trimmed[contentStart:index]), false, true
		}
	}

	return "", false, false
}

func normalizeTOMLKeyPath(path string) string {
	parts := make([]string, 0, 4)
	start := 0

	var quote byte
	escaped := false

	for index := 0; index <= len(path); index++ {
		character := byte(0)
		if index < len(path) {
			character = path[index]
		}

		if quote != 0 {
			if quote == '"' {
				if escaped {
					escaped = false
				} else if character == '\\' {
					escaped = true
				} else if character == quote {
					quote = 0
				}
			} else if character == quote {
				quote = 0
			}
		} else {
			switch character {
			case '"', '\'':
				quote = character
			case '.':
				parts = append(parts, normalizeTOMLKey(path[start:index]))
				start = index + 1
			case 0:
				parts = append(parts, normalizeTOMLKey(path[start:index]))
			}
		}
	}

	return strings.Join(parts, ".")
}

func normalizeTOMLKey(key string) string {
	key = strings.TrimSpace(key)

	if len(key) < 2 {
		return key
	}

	if key[0] == '"' {
		if value, err := strconv.Unquote(key); err == nil {
			return value
		}
	}

	if key[0] == '\'' && key[len(key)-1] == '\'' {
		return key[1 : len(key)-1]
	}

	return key
}

func splitTOMLAssignment(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}

	var quote byte
	escaped := false

	for index := 0; index < len(line); index++ {
		character := line[index]

		if quote != 0 {
			if quote == '"' {
				if escaped {
					escaped = false
				} else if character == '\\' {
					escaped = true
				} else if character == quote {
					quote = 0
				}
			} else if character == quote {
				quote = 0
			}

			continue
		}

		switch character {
		case '"', '\'':
			quote = character
		case '=':
			return normalizeTOMLKeyPath(line[:index]), strings.TrimSpace(line[index+1:]), true
		}
	}

	return "", "", false
}

func tomlAssignments(lines []string, start, end int) []tomlAssignment {
	if end <= start {
		return nil
	}

	assignments := make([]tomlAssignment, 0, end-start)

	for index := start; index < end; index++ {
		key, value, ok := splitTOMLAssignment(lines[index])
		if !ok {
			continue
		}

		last := index
		combined := value

		for !tomlValueComplete(combined) && last+1 < end {
			last++
			combined += "\n" + strings.TrimSpace(lines[last])
		}

		assignments = append(assignments, tomlAssignment{
			key:   key,
			value: combined,
			start: index,
			end:   last + 1,
		})

		index = last
	}

	return assignments
}

func tomlValueComplete(value string) bool {
	depth := 0

	for index := 0; index < len(value); index++ {
		switch value[index] {
		case '#':
			index = tomlCommentEnd(value, index)
		case '"', '\'':
			end, closed := tomlStringEnd(value, index)
			if !closed {
				return false
			}

			index = end
		case '[', '{':
			depth++
		case ']', '}':
			if depth > 0 {
				depth--
			}
		}
	}

	return depth == 0
}

func stripTOMLComment(value string) string {
	for index := 0; index < len(value); index++ {
		switch value[index] {
		case '#':
			return strings.TrimSpace(value[:index])
		case '"', '\'':
			end, closed := tomlStringEnd(value, index)
			if !closed {
				return strings.TrimSpace(value)
			}

			index = end
		}
	}

	return strings.TrimSpace(value)
}

// tomlStringEnd returns the index of the closing quote. index points at the
// opening quote.
func tomlStringEnd(value string, index int) (int, bool) {
	quote := value[index]
	triple := tomlQuoteLength(value, index) == 3

	if triple {
		terminator := strings.Repeat(string(quote), 3)
		offset := strings.Index(value[index+3:], terminator)
		if offset < 0 {
			return len(value) - 1, false
		}

		return index + 3 + offset + 2, true
	}

	escaped := false

	for index++; index < len(value); index++ {
		character := value[index]

		if quote == '"' {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == quote {
				return index, true
			}
		} else if character == quote {
			return index, true
		}
	}

	return len(value) - 1, false
}

func tomlQuoteLength(value string, index int) int {
	if index+2 < len(value) && value[index+1] == value[index] && value[index+2] == value[index] {
		return 3
	}

	return 1
}

func tomlCommentEnd(value string, index int) int {
	for index++; index < len(value); index++ {
		if value[index] == '\n' {
			return index
		}
	}

	return len(value) - 1
}

func parseTOMLString(value string) (string, bool) {
	value = stripTOMLComment(value)

	if len(value) < 2 {
		return "", false
	}

	switch value[0] {
	case '"':
		decoded, err := strconv.Unquote(value)
		if err != nil {
			return "", false
		}

		return decoded, true
	case '\'':
		if value[len(value)-1] != '\'' {
			return "", false
		}

		return value[1 : len(value)-1], true
	default:
		return "", false
	}
}

func parseTOMLStringArray(value string) ([]string, bool) {
	value = strings.TrimSpace(value)

	if !strings.HasPrefix(value, "[") {
		return nil, false
	}

	result := make([]string, 0, 4)
	index := 1

	for {
		index = skipTOMLSpaceAndComments(value, index)
		if index >= len(value) {
			return nil, false
		}

		if value[index] == ']' {
			index++

			break
		}

		parsed, next, ok := parseTOMLStringAt(value, index)
		if !ok {
			return nil, false
		}

		result = append(result, parsed)
		index = skipTOMLSpaceAndComments(value, next)

		if index >= len(value) {
			return nil, false
		}

		switch value[index] {
		case ',':
			index++
		case ']':
			index++
			goto complete
		default:
			return nil, false
		}
	}

complete:
	index = skipTOMLSpaceAndComments(value, index)

	return result, index == len(value)
}

func skipTOMLSpaceAndComments(value string, index int) int {
	for index < len(value) {
		switch value[index] {
		case ' ', '\t', '\r', '\n':
			index++
		case '#':
			for index < len(value) && value[index] != '\n' {
				index++
			}
		default:
			return index
		}
	}

	return index
}

func parseTOMLStringAt(value string, index int) (string, int, bool) {
	if index >= len(value) {
		return "", index, false
	}

	quote := value[index]
	if quote != '"' && quote != '\'' {
		return "", index, false
	}

	start := index
	index++

	escaped := false

	for index < len(value) {
		character := value[index]

		if quote == '"' {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == quote {
				decoded, err := strconv.Unquote(value[start : index+1])
				if err != nil {
					return "", index, false
				}

				return decoded, index + 1, true
			}
		} else if character == quote {
			return value[start+1 : index], index + 1, true
		}

		index++
	}

	return "", index, false
}

func tomlQuote(value string) string {
	var quoted strings.Builder

	quoted.WriteByte('"')

	for _, character := range value {
		switch character {
		case '\\':
			quoted.WriteString(`\\`)
		case '"':
			quoted.WriteString(`\"`)
		case '\b':
			quoted.WriteString(`\b`)
		case '\t':
			quoted.WriteString(`\t`)
		case '\n':
			quoted.WriteString(`\n`)
		case '\f':
			quoted.WriteString(`\f`)
		case '\r':
			quoted.WriteString(`\r`)
		default:
			if character < 0x20 || character == 0x7f {
				fmt.Fprintf(&quoted, `\u%04X`, character)
			} else {
				quoted.WriteRune(character)
			}
		}
	}

	quoted.WriteByte('"')

	return quoted.String()
}

func tomlStringArray(values []string) string {
	quoted := make([]string, 0, len(values))

	for _, value := range values {
		quoted = append(quoted, tomlQuote(value))
	}

	return "[" + strings.Join(quoted, ", ") + "]"
}
