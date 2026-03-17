//go:build !yq_notoon

package yqlib

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
	"strings"
)

type toonDecoder struct {
	reader  *bufio.Reader
	lines   []parsedLine
	pos     int
	indent  int
	delimit string
}

type parsedLine struct {
	content  string
	depth    int
	lineNum  int
	raw      string
}

// NewToonDecoder creates a new TOON decoder.
func NewToonDecoder() Decoder {
	return &toonDecoder{
		indent:  2,
		delimit: ",",
	}
}

func (dec *toonDecoder) Init(reader io.Reader) error {
	dec.reader = bufio.NewReader(reader)
	dec.lines = nil
	dec.pos = 0
	return nil
}

func (dec *toonDecoder) Decode() (*CandidateNode, error) {
	// Read all lines on first decode
	if dec.lines == nil {
		if err := dec.readAllLines(); err != nil {
			return nil, err
		}
	}

	if dec.pos >= len(dec.lines) {
		return nil, io.EOF
	}

	node := dec.decodeValue(0)
	return node, nil
}

func (dec *toonDecoder) readAllLines() error {
	lineNum := 0
	for {
		line, err := dec.reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}

		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		if strings.TrimSpace(line) != "" {
			depth := dec.computeDepth(line)
			dec.lines = append(dec.lines, parsedLine{
				content:  strings.TrimSpace(line),
				depth:    depth,
				lineNum:  lineNum,
				raw:      line,
			})
		}

		lineNum++

		if err == io.EOF {
			break
		}
	}

	return nil
}

func (dec *toonDecoder) computeDepth(line string) int {
	spaceCount := 0
	for _, c := range line {
		if c == ' ' {
			spaceCount++
		} else if c == '\t' {
			spaceCount += dec.indent // Treat tab as indent spaces
		} else {
			break
		}
	}
	return spaceCount / dec.indent
}

func (dec *toonDecoder) peek() *parsedLine {
	if dec.pos >= len(dec.lines) {
		return nil
	}
	return &dec.lines[dec.pos]
}

func (dec *toonDecoder) advance() *parsedLine {
	if dec.pos >= len(dec.lines) {
		return nil
	}
	line := &dec.lines[dec.pos]
	dec.pos++
	return line
}

func (dec *toonDecoder) decodeValue(baseDepth int) *CandidateNode {
	line := dec.peek()
	if line == nil {
		return &CandidateNode{Kind: MappingNode, Tag: "!!map"}
	}

	// Check for root array (starts with [)
	trimmed := strings.TrimSpace(line.content)
	if strings.HasPrefix(trimmed, "[") {
		dec.advance()
		return dec.decodeArrayFromHeader(line.content, baseDepth)
	}

	// Check for single primitive (no key-value)
	if !strings.Contains(line.content, ":") {
		dec.advance()
		primitive := dec.parsePrimitiveToken(line.content)
		return primitive
	}

	// Root object (or key-value pair with array value)
	return dec.decodeObject(baseDepth)
}

func (dec *toonDecoder) decodeObject(baseDepth int) *CandidateNode {
	node := &CandidateNode{
		Kind: MappingNode,
		Tag:  "!!map",
	}

	for {
		line := dec.peek()
		if line == nil || line.depth < baseDepth {
			break
		}

		if line.depth != baseDepth {
			break
		}

		dec.advance()
		dec.decodeKeyValue(line.content, node, baseDepth)
	}

	return node
}

func (dec *toonDecoder) decodeKeyValue(content string, parent *CandidateNode, baseDepth int) {
	// Check for array header in key-value pair (e.g., "tags[3]: admin,ops,dev")
	header := dec.parseArrayHeaderLine(content)
	if header != nil && header.key != "" {
		keyNode := &CandidateNode{Kind: ScalarNode, Tag: "!!str", Value: header.key}
		valueNode := dec.decodeArrayFromHeaderInfo(header, baseDepth)
		parent.Content = append(parent.Content, keyNode, valueNode)
		return
	}

	// Regular key-value pair
	key, rest := dec.parseKey(content)
	keyNode := &CandidateNode{Kind: ScalarNode, Tag: "!!str", Value: key}

	if rest == "" {
		// No value after colon - check for nested object
		nextLine := dec.peek()
		if nextLine != nil && nextLine.depth > baseDepth {
			valueNode := dec.decodeObject(nextLine.depth)
			parent.Content = append(parent.Content, keyNode, valueNode)
			return
		}
		// Empty object
		emptyObj := &CandidateNode{Kind: MappingNode, Tag: "!!map"}
		parent.Content = append(parent.Content, keyNode, emptyObj)
		return
	}

	// Inline primitive value
	valueNode := dec.parsePrimitiveToken(rest)
	parent.Content = append(parent.Content, keyNode, valueNode)
}

func (dec *toonDecoder) parseKey(content string) (string, string) {
	// Handle quoted key
	if strings.HasPrefix(content, "\"") {
		endQuote := findClosingQuote(content, 0)
		if endQuote == -1 {
			return content, ""
		}
		key := dec.unescapeString(content[1:endQuote])
		rest := ""
		if endQuote+1 < len(content) {
			afterQuote := content[endQuote+1:]
			colonIdx := strings.Index(afterQuote, ":")
			if colonIdx != -1 {
				rest = strings.TrimSpace(afterQuote[colonIdx+1:])
			}
		}
		return key, rest
	}

	// Unquoted key
	colonIdx := strings.Index(content, ":")
	if colonIdx == -1 {
		return content, ""
	}
	key := strings.TrimSpace(content[:colonIdx])
	rest := strings.TrimSpace(content[colonIdx+1:])
	return key, rest
}

// arrayHeaderInfo holds parsed array header information
type arrayHeaderInfo struct {
	key       string
	length    int
	delimiter string
	fields    []string
	inline    string // inline values after colon
}

func isArrayHeaderContent(content string) bool {
	trimmed := strings.TrimSpace(content)
	// Root array: starts with [
	if strings.HasPrefix(trimmed, "[") {
		return true
	}
	// Not an array header if it has a key before the [
	// (e.g., "tags[3]: values" is a key-value pair, not a root array)
	return false
}

func (dec *toonDecoder) parseArrayHeaderLine(content string) *arrayHeaderInfo {
	info := &arrayHeaderInfo{delimiter: ","}

	// Find the bracket
	bracketStart := strings.Index(content, "[")
	if bracketStart == -1 {
		return nil
	}

	bracketEnd := strings.Index(content[bracketStart:], "]")
	if bracketEnd == -1 {
		return nil
	}
	bracketEnd += bracketStart

	// Extract key if present
	if bracketStart > 0 {
		key := strings.TrimSpace(content[:bracketStart])
		if strings.HasPrefix(key, "\"") {
			endQuote := findClosingQuote(key, 0)
			if endQuote != -1 {
				info.key = dec.unescapeString(key[1:endQuote])
			}
		} else {
			info.key = key
		}
	}

	// Find colon after bracket
	colonIdx := strings.Index(content[bracketEnd:], ":")
	if colonIdx == -1 {
		return nil
	}
	colonIdx += bracketEnd

	// Parse bracket content
	bracketContent := content[bracketStart+1 : bracketEnd]
	info.length, info.delimiter = dec.parseBracketSegment(bracketContent)

	// Check for fields segment
	braceStart := strings.Index(content[bracketEnd:colonIdx], "{")
	if braceStart != -1 {
		braceStart += bracketEnd
		braceEnd := strings.Index(content[braceStart:], "}")
		if braceEnd != -1 {
			braceEnd += braceStart
			fieldsContent := content[braceStart+1 : braceEnd]
			info.fields = dec.parseDelimitedValues(fieldsContent, info.delimiter)
		}
	}

	// Extract inline values
	if colonIdx+1 < len(content) {
		info.inline = strings.TrimSpace(content[colonIdx+1:])
	}

	return info
}

func (dec *toonDecoder) parseBracketSegment(seg string) (int, string) {
	delimiter := ","

	// Check for delimiter suffix
	if strings.HasSuffix(seg, "\t") {
		delimiter = "\t"
		seg = seg[:len(seg)-1]
	} else if strings.HasSuffix(seg, "|") {
		delimiter = "|"
		seg = seg[:len(seg)-1]
	}

	length, _ := strconv.Atoi(seg)
	return length, delimiter
}

func (dec *toonDecoder) parseDelimitedValues(input string, delimiter string) []string {
	var values []string
	var valueBuf strings.Builder
	inQuotes := false

	for i := 0; i < len(input); i++ {
		c := input[i]

		if c == '\\' && i+1 < len(input) && inQuotes {
			valueBuf.WriteByte(c)
			valueBuf.WriteByte(input[i+1])
			i++
			continue
		}

		if c == '"' {
			inQuotes = !inQuotes
			valueBuf.WriteByte(c)
			continue
		}

		if string(c) == delimiter && !inQuotes {
			values = append(values, strings.TrimSpace(valueBuf.String()))
			valueBuf.Reset()
			continue
		}

		valueBuf.WriteByte(c)
	}

	// Add last value
	if valueBuf.Len() > 0 || len(values) > 0 {
		values = append(values, strings.TrimSpace(valueBuf.String()))
	}

	return values
}

func (dec *toonDecoder) decodeArrayFromHeader(content string, baseDepth int) *CandidateNode {
	header := dec.parseArrayHeaderLine(content)
	return dec.decodeArrayFromHeaderInfo(header, baseDepth)
}

func (dec *toonDecoder) decodeArrayFromHeaderInfo(header *arrayHeaderInfo, baseDepth int) *CandidateNode {
	node := &CandidateNode{
		Kind: SequenceNode,
		Tag:  "!!seq",
	}

	// Inline primitive array
	if header.inline != "" {
		values := dec.parseDelimitedValues(header.inline, header.delimiter)
		for _, v := range values {
			primitive := dec.parsePrimitiveToken(v)
			node.Content = append(node.Content, primitive)
		}
		return node
	}

	// Tabular array
	if len(header.fields) > 0 {
		return dec.decodeTabularArray(header, baseDepth, node)
	}

	// List array
	return dec.decodeListArray(header, baseDepth, node)
}

func (dec *toonDecoder) decodeTabularArray(header *arrayHeaderInfo, baseDepth int, node *CandidateNode) *CandidateNode {
	rowDepth := baseDepth + 1
	rowCount := 0

	for rowCount < header.length {
		line := dec.peek()
		if line == nil || line.depth < rowDepth {
			break
		}

		if line.depth == rowDepth {
			dec.advance()
			values := dec.parseDelimitedValues(line.content, header.delimiter)

			// Create object from fields and values
			obj := &CandidateNode{Kind: MappingNode, Tag: "!!map"}
			for i, field := range header.fields {
				keyNode := &CandidateNode{Kind: ScalarNode, Tag: "!!str", Value: field}
				var valueNode *CandidateNode
				if i < len(values) {
					valueNode = dec.parsePrimitiveToken(values[i])
				} else {
					valueNode = &CandidateNode{Kind: ScalarNode, Tag: "!!null", Value: "null"}
				}
				obj.Content = append(obj.Content, keyNode, valueNode)
			}
			node.Content = append(node.Content, obj)
			rowCount++
		} else {
			break
		}
	}

	return node
}

func (dec *toonDecoder) decodeListArray(header *arrayHeaderInfo, baseDepth int, node *CandidateNode) *CandidateNode {
	itemDepth := baseDepth + 1
	itemCount := 0

	for itemCount < header.length {
		line := dec.peek()
		if line == nil || line.depth < itemDepth {
			break
		}

		if line.depth == itemDepth && (strings.HasPrefix(line.content, "- ") || line.content == "-") {
			dec.advance()
			item := dec.decodeListItem(line.content, itemDepth)
			node.Content = append(node.Content, item)
			itemCount++
		} else {
			break
		}
	}

	return node
}

func (dec *toonDecoder) decodeListItem(content string, baseDepth int) *CandidateNode {
	// Bare list item marker
	if content == "-" {
		return &CandidateNode{Kind: MappingNode, Tag: "!!map"}
	}

	afterHyphen := strings.TrimPrefix(content, "- ")
	afterHyphen = strings.TrimSpace(afterHyphen)

	if afterHyphen == "" {
		return &CandidateNode{Kind: MappingNode, Tag: "!!map"}
	}

	// Check for array header after hyphen
	if isArrayHeaderContent(afterHyphen) {
		header := dec.parseArrayHeaderLine(afterHyphen)
		return dec.decodeArrayFromHeaderInfo(header, baseDepth)
	}

	// Check for object first field after hyphen
	if strings.Contains(afterHyphen, ":") {
		return dec.decodeObjectListItem(afterHyphen, baseDepth)
	}

	// Primitive value
	return dec.parsePrimitiveToken(afterHyphen)
}

func (dec *toonDecoder) decodeObjectListItem(content string, baseDepth int) *CandidateNode {
	node := &CandidateNode{Kind: MappingNode, Tag: "!!map"}

	// Parse first key-value
	key, rest := dec.parseKey(content)
	keyNode := &CandidateNode{Kind: ScalarNode, Tag: "!!str", Value: key}

	if rest == "" {
		// Check for nested object or array
		nextLine := dec.peek()
		if nextLine != nil && nextLine.depth > baseDepth {
			// Check if it's an array header
			if isArrayHeaderContent(nextLine.content) {
				dec.advance()
				valueNode := dec.decodeArrayFromHeader(nextLine.content, nextLine.depth)
				node.Content = append(node.Content, keyNode, valueNode)
			} else {
				valueNode := dec.decodeObject(nextLine.depth)
				node.Content = append(node.Content, keyNode, valueNode)
			}
		} else {
			emptyObj := &CandidateNode{Kind: MappingNode, Tag: "!!map"}
			node.Content = append(node.Content, keyNode, emptyObj)
		}
	} else {
		// Check if rest is an array header
		if isArrayHeaderContent(rest) {
			header := dec.parseArrayHeaderLine(rest)
			valueNode := dec.decodeArrayFromHeaderInfo(header, baseDepth+1)
			node.Content = append(node.Content, keyNode, valueNode)
		} else {
			valueNode := dec.parsePrimitiveToken(rest)
			node.Content = append(node.Content, keyNode, valueNode)
		}
	}

	// Read subsequent fields at baseDepth + 1
	followDepth := baseDepth + 1
	for {
		nextLine := dec.peek()
		if nextLine == nil || nextLine.depth < followDepth {
			break
		}

		if nextLine.depth == followDepth && !strings.HasPrefix(nextLine.content, "-") {
			dec.advance()
			dec.decodeKeyValue(nextLine.content, node, followDepth)
		} else {
			break
		}
	}

	return node
}

func (dec *toonDecoder) parsePrimitiveToken(token string) *CandidateNode {
	token = strings.TrimSpace(token)

	if token == "" {
		return &CandidateNode{Kind: ScalarNode, Tag: "!!str", Value: ""}
	}

	// Quoted string
	if strings.HasPrefix(token, "\"") {
		endQuote := findClosingQuote(token, 0)
		if endQuote == len(token)-1 {
			return &CandidateNode{
				Kind:  ScalarNode,
				Tag:   "!!str",
				Value: dec.unescapeString(token[1:endQuote]),
			}
		}
	}

	// Boolean literals
	lower := strings.ToLower(token)
	if lower == "true" {
		return &CandidateNode{Kind: ScalarNode, Tag: "!!bool", Value: "true"}
	}
	if lower == "false" {
		return &CandidateNode{Kind: ScalarNode, Tag: "!!bool", Value: "false"}
	}

	// Null literal
	if lower == "null" {
		return &CandidateNode{Kind: ScalarNode, Tag: "!!null", Value: "null"}
	}

	// Numeric literal
	if dec.isNumericLiteral(token) {
		// Determine if int or float
		if strings.ContainsAny(token, ".eE") {
			return &CandidateNode{Kind: ScalarNode, Tag: "!!float", Value: token}
		}
		return &CandidateNode{Kind: ScalarNode, Tag: "!!int", Value: token}
	}

	// Unquoted string
	return &CandidateNode{Kind: ScalarNode, Tag: "!!str", Value: token}
}

var numericPattern = regexp.MustCompile(`^-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?$`)

func (dec *toonDecoder) isNumericLiteral(token string) bool {
	return numericPattern.MatchString(token)
}

func (dec *toonDecoder) unescapeString(value string) string {
	var result strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == '\\' && i+1 < len(value) {
			next := value[i+1]
			switch next {
			case 'n':
				result.WriteByte('\n')
			case 't':
				result.WriteByte('\t')
			case 'r':
				result.WriteByte('\r')
			case '\\':
				result.WriteByte('\\')
			case '"':
				result.WriteByte('"')
			default:
				result.WriteByte('\\')
				result.WriteByte(next)
			}
			i++
		} else {
			result.WriteByte(value[i])
		}
	}
	return result.String()
}

func findClosingQuote(content string, start int) int {
	i := start + 1
	for i < len(content) {
		if content[i] == '\\' && i+1 < len(content) {
			i += 2
			continue
		}
		if content[i] == '"' {
			return i
		}
		i++
	}
	return -1
}
