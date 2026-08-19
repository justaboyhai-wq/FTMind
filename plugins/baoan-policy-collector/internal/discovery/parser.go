package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/model"
)

func ParseAllData(src []byte) ([]model.IndexRecord, error) {
	literal, err := extractAssignedArray(src, "allData")
	if err != nil {
		return nil, err
	}
	p := &literalParser{src: literal}
	v, err := p.value()
	if err != nil {
		return nil, err
	}
	p.space()
	if p.pos != len(p.src) {
		return nil, fmt.Errorf("unexpected token at %d", p.pos)
	}
	normalized, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out []model.IndexRecord
	if err := json.Unmarshal(normalized, &out); err != nil {
		return nil, fmt.Errorf("decode allData: %w", err)
	}
	if err := validateUniqueIDs(out); err != nil {
		return nil, err
	}
	return out, nil
}

func extractAssignedArray(src []byte, name string) ([]byte, error) {
	needle := []byte(name)
	for start := 0; ; {
		i := bytes.Index(src[start:], needle)
		if i < 0 {
			return nil, fmt.Errorf("assignment %s not found", name)
		}
		i += start
		if (i > 0 && isIdent(src[i-1])) || (i+len(needle) < len(src) && isIdent(src[i+len(needle)])) {
			start = i + len(needle)
			continue
		}
		j := i + len(needle)
		for j < len(src) && (src[j] == ' ' || src[j] == '\t' || src[j] == '\r' || src[j] == '\n') {
			j++
		}
		if j >= len(src) || src[j] != '=' {
			start = i + len(needle)
			continue
		}
		j++
		for j < len(src) && (src[j] == ' ' || src[j] == '\t' || src[j] == '\r' || src[j] == '\n') {
			j++
		}
		if j >= len(src) || src[j] != '[' {
			start = i + len(needle)
			continue
		}
		end, err := balancedEnd(src, j, '[', ']')
		if err != nil {
			return nil, err
		}
		return src[j : end+1], nil
	}
}

func balancedEnd(src []byte, start int, open, close byte) (int, error) {
	depth := 0
	for i := int(start); i < len(src); i++ {
		switch src[i] {
		case '\'', '"':
			n, err := skipString(src, i)
			if err != nil {
				return 0, err
			}
			i = n
		case '/':
			if i+1 < len(src) && src[i+1] == '/' {
				for i < len(src) && src[i] != '\n' {
					i++
				}
				continue
			}
			if i+1 < len(src) && src[i+1] == '*' {
				n := bytes.Index(src[i+2:], []byte("*/"))
				if n < 0 {
					return 0, fmt.Errorf("unterminated comment")
				}
				i += n + 3
				continue
			}
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return 0, fmt.Errorf("unclosed allData array")
}

func skipString(src []byte, start int) (int, error) {
	quote := src[start]
	for i := start + 1; i < len(src); i++ {
		if src[i] == '\\' {
			i++
			continue
		}
		if src[i] == quote {
			return i, nil
		}
		if src[i] == '\n' || src[i] == '\r' {
			return 0, fmt.Errorf("newline in string")
		}
	}
	return 0, fmt.Errorf("unterminated string")
}

func isIdent(b byte) bool {
	return b == '_' || b == '$' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

type literalParser struct {
	src []byte
	pos int
}

func (p *literalParser) space() {
	for p.pos < len(p.src) {
		if strings.ContainsRune(" \t\r\n", rune(p.src[p.pos])) {
			p.pos++
			continue
		}
		if p.pos+1 < len(p.src) && p.src[p.pos : p.pos+2][0] == '/' && p.src[p.pos : p.pos+2][1] == '/' {
			for p.pos < len(p.src) && p.src[p.pos] != '\n' {
				p.pos++
			}
			continue
		}
		if p.pos+1 < len(p.src) && p.src[p.pos : p.pos+2][0] == '/' && p.src[p.pos : p.pos+2][1] == '*' {
			n := bytes.Index(p.src[p.pos+2:], []byte("*/"))
			if n < 0 {
				p.pos = len(p.src)
			} else {
				p.pos += n + 4
			}
			continue
		}
		break
	}
}
func (p *literalParser) value() (any, error) {
	p.space()
	if p.pos >= len(p.src) {
		return nil, fmt.Errorf("value expected")
	}
	switch p.src[p.pos] {
	case '{':
		return p.object()
	case '[':
		return p.array()
	case '\'', '"':
		return p.string()
	case 't':
		if p.take("true") {
			return true, nil
		}
	case 'f':
		if p.take("false") {
			return false, nil
		}
	case 'n':
		if p.take("null") {
			return nil, nil
		}
	}
	return p.number()
}
func (p *literalParser) take(s string) bool {
	if bytes.HasPrefix(p.src[p.pos:], []byte(s)) {
		p.pos += len(s)
		return true
	}
	return false
}
func (p *literalParser) object() (map[string]any, error) {
	out := map[string]any{}
	p.pos++
	p.space()
	if p.pos < len(p.src) && p.src[p.pos] == '}' {
		p.pos++
		return out, nil
	}
	for {
		p.space()
		if p.pos >= len(p.src) || (p.src[p.pos] != '\'' && p.src[p.pos] != '"') {
			return nil, fmt.Errorf("object key expected at %d", p.pos)
		}
		k, err := p.string()
		if err != nil {
			return nil, err
		}
		p.space()
		if p.pos >= len(p.src) || p.src[p.pos] != ':' {
			return nil, fmt.Errorf("colon expected at %d", p.pos)
		}
		p.pos++
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		out[k.(string)] = v
		p.space()
		if p.pos >= len(p.src) {
			return nil, fmt.Errorf("object not closed")
		}
		if p.src[p.pos] == '}' {
			p.pos++
			return out, nil
		}
		if p.src[p.pos] != ',' {
			return nil, fmt.Errorf("comma expected at %d", p.pos)
		}
		p.pos++
		p.space()
		if p.pos < len(p.src) && p.src[p.pos] == '}' {
			p.pos++
			return out, nil
		}
	}
}
func (p *literalParser) array() ([]any, error) {
	out := []any{}
	p.pos++
	p.space()
	if p.pos < len(p.src) && p.src[p.pos] == ']' {
		p.pos++
		return out, nil
	}
	for {
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
		p.space()
		if p.pos >= len(p.src) {
			return nil, fmt.Errorf("array not closed")
		}
		if p.src[p.pos] == ']' {
			p.pos++
			return out, nil
		}
		if p.src[p.pos] != ',' {
			return nil, fmt.Errorf("comma expected at %d", p.pos)
		}
		p.pos++
		p.space()
		if p.pos < len(p.src) && p.src[p.pos] == ']' {
			p.pos++
			return out, nil
		}
	}
}
func (p *literalParser) string() (any, error) {
	quote := p.src[p.pos]
	p.pos++
	var b strings.Builder
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		p.pos++
		if c == quote {
			return b.String(), nil
		}
		if c != '\\' {
			if c >= utf8.RuneSelf {
				r, n := utf8.DecodeRune(p.src[p.pos-1:])
				if r == utf8.RuneError && n == 1 {
					return nil, fmt.Errorf("invalid UTF-8")
				}
				b.WriteRune(r)
				p.pos += n - 1
			} else {
				b.WriteByte(c)
			}
			continue
		}
		if p.pos >= len(p.src) {
			break
		}
		e := p.src[p.pos]
		p.pos++
		switch e {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case '\\', '/', '\'', '"':
			b.WriteByte(e)
		case 'u':
			if p.pos+4 > len(p.src) {
				return nil, fmt.Errorf("short unicode escape")
			}
			n, err := strconv.ParseUint(string(p.src[p.pos:p.pos+4]), 16, 16)
			if err != nil {
				return nil, err
			}
			b.WriteRune(rune(n))
			p.pos += 4
		case 'x':
			if p.pos+2 > len(p.src) {
				return nil, fmt.Errorf("short hex escape")
			}
			n, err := strconv.ParseUint(string(p.src[p.pos:p.pos+2]), 16, 8)
			if err != nil {
				return nil, err
			}
			b.WriteByte(byte(n))
			p.pos += 2
		default:
			return nil, fmt.Errorf("unsupported escape \\%c", e)
		}
	}
	return nil, fmt.Errorf("unterminated string")
}
func (p *literalParser) number() (any, error) {
	start := p.pos
	if p.pos < len(p.src) && (p.src[p.pos] == '-' || p.src[p.pos] == '+') {
		p.pos++
	}
	for p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
		p.pos++
	}
	if p.pos < len(p.src) && p.src[p.pos] == '.' {
		p.pos++
		for p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
			p.pos++
		}
	}
	if p.pos < len(p.src) && (p.src[p.pos] == 'e' || p.src[p.pos] == 'E') {
		p.pos++
		if p.pos < len(p.src) && (p.src[p.pos] == '-' || p.src[p.pos] == '+') {
			p.pos++
		}
		for p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
			p.pos++
		}
	}
	raw := string(p.src[start:p.pos])
	if raw == "" || raw == "+" || raw == "-" {
		return nil, fmt.Errorf("unexpected token at %d", start)
	}
	if strings.ContainsAny(raw, ".eE") {
		v, err := strconv.ParseFloat(raw, 64)
		return v, err
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	return v, err
}
func validateUniqueIDs(records []model.IndexRecord) error {
	seen := map[string]bool{}
	for _, r := range records {
		if r.ID == "" {
			return fmt.Errorf("empty record id")
		}
		if seen[r.ID] {
			return fmt.Errorf("duplicate record id %s", r.ID)
		}
		seen[r.ID] = true
	}
	if len(records) == 0 {
		return fmt.Errorf("allData is empty")
	}
	return nil
}
