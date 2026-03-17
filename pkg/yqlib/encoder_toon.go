//go:build !yq_notoon

package yqlib

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

// ToonPreferences holds configuration options for TOON encoding.
type ToonPreferences struct {
	Indent        int
	Delimiter     string // ",", "\t", or "|"
	ColorsEnabled bool
}

// ConfiguredToonPreferences is the global TOON preferences instance.
var ConfiguredToonPreferences = ToonPreferences{
	Indent:        2,
	Delimiter:     ",",
	ColorsEnabled: true,
}

type toonEncoder struct {
	prefs ToonPreferences
}

// NewToonEncoder creates a new TOON encoder with the given preferences.
func NewToonEncoder(prefs ToonPreferences) Encoder {
	return &toonEncoder{prefs: prefs}
}

func (te *toonEncoder) CanHandleAliases() bool {
	return false
}

func (te *toonEncoder) PrintDocumentSeparator(_ io.Writer) error {
	return nil
}

func (te *toonEncoder) PrintLeadingContent(_ io.Writer, _ string) error {
	return nil
}

func (te *toonEncoder) Encode(writer io.Writer, node *CandidateNode) error {
	log.Debugf("TOON encoding %v", NodeToString(node))

	destination := writer
	tempBuffer := bytes.NewBuffer(nil)
	if te.prefs.ColorsEnabled {
		destination = tempBuffer
	}

	// Handle scalar at root level
	switch node.Kind {
	case ScalarNode:
		encoded := te.encodePrimitive(node)
		if te.prefs.ColorsEnabled {
			return colourizeToonAndPrint([]byte(encoded+"\n"), writer)
		}
		return writeString(destination, encoded+"\n")
	case SequenceNode:
		if err := te.encodeArrayLines(destination, "", node, 0); err != nil {
			return err
		}
		if te.prefs.ColorsEnabled {
			return colourizeToonAndPrint(tempBuffer.Bytes(), writer)
		}
		return nil
	case MappingNode:
		if err := te.encodeObjectLines(destination, node, 0); err != nil {
			return err
		}
		if te.prefs.ColorsEnabled {
			return colourizeToonAndPrint(tempBuffer.Bytes(), writer)
		}
		return nil
	}
	return nil
}

func (te *toonEncoder) encodePrimitive(node *CandidateNode) string {
	tag := node.guessTagFromCustomType()

	switch tag {
	case "!!null":
		return "null"
	case "!!bool":
		return node.Value
	case "!!int", "!!float":
		return node.Value
	default:
		return te.encodeStringLiteral(node.Value)
	}
}

func (te *toonEncoder) encodeStringLiteral(value string) string {
	if te.isSafeUnquoted(value) {
		return value
	}
	return "\"" + te.escapeString(value) + "\""
}

func (te *toonEncoder) isSafeUnquoted(value string) bool {
	if value == "" {
		return false
	}

	// Has leading or trailing whitespace
	if value != strings.TrimSpace(value) {
		return false
	}

	// Check if it looks like a literal (boolean, null, number)
	if te.isBooleanOrNullLiteral(value) || te.isNumericLike(value) {
		return false
	}

	// Check for colon (always structural)
	if strings.Contains(value, ":") {
		return false
	}

	// Check for quotes and backslash (always need escaping)
	if strings.Contains(value, "\"") || strings.Contains(value, "\\") {
		return false
	}

	// Check for brackets and braces (always structural)
	if strings.ContainsAny(value, "[]{}") {
		return false
	}

	// Check for control characters
	if strings.ContainsAny(value, "\n\r\t") {
		return false
	}

	// Check for the active delimiter
	if strings.Contains(value, te.prefs.Delimiter) {
		return false
	}

	// Check for hyphen at start (list marker)
	if strings.HasPrefix(value, "-") {
		return false
	}

	return true
}

func (te *toonEncoder) isBooleanOrNullLiteral(value string) bool {
	lower := strings.ToLower(value)
	return lower == "true" || lower == "false" || lower == "null"
}

func (te *toonEncoder) isNumericLike(value string) bool {
	if len(value) == 0 {
		return false
	}
	// Simple numeric pattern check
	// Match numbers like 42, -3.14, 1e-6, etc.
	start := 0
	if value[0] == '-' {
		start = 1
	}
	if start >= len(value) {
		return false
	}

	hasDigit := false
	hasDot := false
	hasExp := false

	for i := start; i < len(value); i++ {
		c := value[i]
		if c >= '0' && c <= '9' {
			hasDigit = true
			continue
		}
		if c == '.' && !hasDot && !hasExp {
			hasDot = true
			continue
		}
		if (c == 'e' || c == 'E') && !hasExp {
			hasExp = true
			// Check for optional sign after exponent
			if i+1 < len(value) && (value[i+1] == '+' || value[i+1] == '-') {
				i++
			}
			continue
		}
		return false
	}

	return hasDigit
}

func (te *toonEncoder) escapeString(value string) string {
	var result strings.Builder
	for _, c := range value {
		switch c {
		case '\\':
			result.WriteString("\\\\")
		case '"':
			result.WriteString("\\\"")
		case '\n':
			result.WriteString("\\n")
		case '\r':
			result.WriteString("\\r")
		case '\t':
			result.WriteString("\\t")
		default:
			result.WriteRune(c)
		}
	}
	return result.String()
}

func (te *toonEncoder) encodeKey(key string) string {
	if te.isValidUnquotedKey(key) {
		return key
	}
	return "\"" + te.escapeString(key) + "\""
}

func (te *toonEncoder) isValidUnquotedKey(key string) bool {
	if len(key) == 0 {
		return false
	}
	// Must start with letter or underscore
	c := key[0]
	if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && c != '_' {
		return false
	}
	// Followed by letters, digits, underscores, or dots
	for i := 1; i < len(key); i++ {
		c := key[i]
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' && c != '.' {
			return false
		}
	}
	return true
}

func (te *toonEncoder) encodeObjectLines(writer io.Writer, node *CandidateNode, depth int) error {
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]
		if err := te.encodeKeyValuePairLines(writer, keyNode.Value, valueNode, depth); err != nil {
			return err
		}
	}
	return nil
}

func (te *toonEncoder) encodeKeyValuePairLines(writer io.Writer, key string, value *CandidateNode, depth int) error {
	encodedKey := te.encodeKey(key)

	switch value.Kind {
	case ScalarNode:
		encodedValue := te.encodePrimitive(value)
		return te.writeIndentedLine(writer, depth, encodedKey+": "+encodedValue)
	case SequenceNode:
		return te.encodeArrayLines(writer, key, value, depth)
	case MappingNode:
		if err := te.writeIndentedLine(writer, depth, encodedKey+":"); err != nil {
			return err
		}
		if len(value.Content) > 0 {
			return te.encodeObjectLines(writer, value, depth+1)
		}
		return nil
	}
	return nil
}

func (te *toonEncoder) encodeArrayLines(writer io.Writer, key string, node *CandidateNode, depth int) error {
	length := len(node.Content)

	if length == 0 {
		header := te.formatHeader(key, length, nil)
		return te.writeIndentedLine(writer, depth, header)
	}

	// Check if all primitives
	if te.isArrayOfPrimitives(node) {
		arrayLine := te.encodeInlineArrayLine(key, node)
		return te.writeIndentedLine(writer, depth, arrayLine)
	}

	// Check if array of objects with uniform fields (tabular)
	if header := te.extractTabularHeader(node); header != nil {
		return te.encodeArrayOfObjectsAsTabular(writer, key, node, header, depth)
	}

	// Mixed or non-uniform array: use list format
	return te.encodeMixedArrayAsListItems(writer, key, node, depth)
}

func (te *toonEncoder) isArrayOfPrimitives(node *CandidateNode) bool {
	for _, child := range node.Content {
		if child.Kind != ScalarNode {
			return false
		}
	}
	return true
}

func (te *toonEncoder) encodeInlineArrayLine(key string, node *CandidateNode) string {
	length := len(node.Content)
	header := te.formatHeader(key, length, nil)

	if length == 0 {
		return header
	}

	var values []string
	for _, child := range node.Content {
		values = append(values, te.encodePrimitive(child))
	}

	return header + " " + strings.Join(values, te.prefs.Delimiter)
}

func (te *toonEncoder) extractTabularHeader(node *CandidateNode) []string {
	if len(node.Content) == 0 {
		return nil
	}

	firstChild := node.Content[0]
	if firstChild.Kind != MappingNode {
		return nil
	}

	// Extract keys from first object
	var header []string
	for i := 0; i < len(firstChild.Content); i += 2 {
		header = append(header, firstChild.Content[i].Value)
	}

	if len(header) == 0 {
		return nil
	}

	// Verify all objects have the same keys and all values are primitives
	for _, child := range node.Content {
		if child.Kind != MappingNode {
			return nil
		}
		if len(child.Content) != len(header)*2 {
			return nil
		}

		// Check all values are primitives
		for j := 0; j < len(child.Content); j += 2 {
			valueNode := child.Content[j+1]
			if valueNode.Kind != ScalarNode {
				return nil
			}
		}

		// Check keys match
		keySet := make(map[string]bool)
		for j := 0; j < len(child.Content); j += 2 {
			keySet[child.Content[j].Value] = true
		}
		for _, k := range header {
			if !keySet[k] {
				return nil
			}
		}
	}

	return header
}

func (te *toonEncoder) encodeArrayOfObjectsAsTabular(writer io.Writer, key string, node *CandidateNode, header []string, depth int) error {
	formattedHeader := te.formatHeader(key, len(node.Content), header)
	if err := te.writeIndentedLine(writer, depth, formattedHeader); err != nil {
		return err
	}

	// Write rows
	for _, child := range node.Content {
		var values []string
		for _, k := range header {
			// Find value for this key
			var val *CandidateNode
			for j := 0; j < len(child.Content); j += 2 {
				if child.Content[j].Value == k {
					val = child.Content[j+1]
					break
				}
			}
			if val != nil {
				values = append(values, te.encodePrimitive(val))
			} else {
				values = append(values, "null")
			}
		}
		if err := te.writeIndentedLine(writer, depth+1, strings.Join(values, te.prefs.Delimiter)); err != nil {
			return err
		}
	}
	return nil
}

func (te *toonEncoder) encodeMixedArrayAsListItems(writer io.Writer, key string, node *CandidateNode, depth int) error {
	header := te.formatHeader(key, len(node.Content), nil)
	if err := te.writeIndentedLine(writer, depth, header); err != nil {
		return err
	}

	for _, item := range node.Content {
		if err := te.encodeListItemValue(writer, item, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func (te *toonEncoder) encodeListItemValue(writer io.Writer, value *CandidateNode, depth int) error {
	switch value.Kind {
	case ScalarNode:
		return te.writeIndentedLine(writer, depth, "- "+te.encodePrimitive(value))
	case SequenceNode:
		if te.isArrayOfPrimitives(value) {
			arrayLine := te.encodeInlineArrayLine("", value)
			return te.writeIndentedLine(writer, depth, "- "+arrayLine)
		}
		header := te.formatHeader("", len(value.Content), nil)
		if err := te.writeIndentedLine(writer, depth, "- "+header); err != nil {
			return err
		}
		for _, item := range value.Content {
			if err := te.encodeListItemValue(writer, item, depth+1); err != nil {
				return err
			}
		}
		return nil
	case MappingNode:
		return te.encodeObjectAsListItem(writer, value, depth)
	}
	return nil
}

func (te *toonEncoder) encodeObjectAsListItem(writer io.Writer, obj *CandidateNode, depth int) error {
	if len(obj.Content) == 0 {
		return te.writeIndentedLine(writer, depth, "-")
	}

	// First key-value pair
	firstKey := obj.Content[0].Value
	firstValue := obj.Content[1]
	encodedKey := te.encodeKey(firstKey)

	switch firstValue.Kind {
	case ScalarNode:
		encodedValue := te.encodePrimitive(firstValue)
		return te.writeIndentedLine(writer, depth, "- "+encodedKey+": "+encodedValue)
	case SequenceNode:
		if len(firstValue.Content) == 0 {
			header := te.formatHeader(firstKey, 0, nil)
			return te.writeIndentedLine(writer, depth, "- "+header)
		}
		if te.isArrayOfPrimitives(firstValue) {
			arrayLine := te.encodeInlineArrayLine(firstKey, firstValue)
			return te.writeIndentedLine(writer, depth, "- "+arrayLine)
		}
		if tabularHeader := te.extractTabularHeader(firstValue); tabularHeader != nil {
			// Tabular array as first field
			formattedHeader := te.formatHeader(firstKey, len(firstValue.Content), tabularHeader)
			if err := te.writeIndentedLine(writer, depth, "- "+formattedHeader); err != nil {
				return err
			}
			for _, child := range firstValue.Content {
				var values []string
				for _, k := range tabularHeader {
					var val *CandidateNode
					for j := 0; j < len(child.Content); j += 2 {
						if child.Content[j].Value == k {
							val = child.Content[j+1]
							break
						}
					}
					if val != nil {
						values = append(values, te.encodePrimitive(val))
					} else {
						values = append(values, "null")
					}
				}
				if err := te.writeIndentedLine(writer, depth+2, strings.Join(values, te.prefs.Delimiter)); err != nil {
					return err
				}
			}
			return nil
		}
		header := te.formatHeader(firstKey, len(firstValue.Content), nil)
		if err := te.writeIndentedLine(writer, depth, "- "+header); err != nil {
			return err
		}
		for _, item := range firstValue.Content {
			if err := te.encodeListItemValue(writer, item, depth+1); err != nil {
				return err
			}
		}
		return nil
	case MappingNode:
		if err := te.writeIndentedLine(writer, depth, "- "+encodedKey+":"); err != nil {
			return err
		}
		if len(firstValue.Content) > 0 {
			return te.encodeObjectLines(writer, firstValue, depth+2)
		}
		return nil
	}

	// Remaining key-value pairs
	for i := 2; i < len(obj.Content); i += 2 {
		key := obj.Content[i].Value
		value := obj.Content[i+1]
		if err := te.encodeKeyValuePairLines(writer, key, value, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func (te *toonEncoder) formatHeader(key string, length int, fields []string) string {
	var header strings.Builder

	if key != "" {
		header.WriteString(te.encodeKey(key))
	}

	header.WriteString("[")
	header.WriteString(strconv.Itoa(length))

	// Add delimiter suffix if not comma
	if te.prefs.Delimiter != "," {
		header.WriteString(te.prefs.Delimiter)
	}

	header.WriteString("]")

	if len(fields) > 0 {
		header.WriteString("{")
		for i, f := range fields {
			if i > 0 {
				header.WriteString(te.prefs.Delimiter)
			}
			header.WriteString(te.encodeKey(f))
		}
		header.WriteString("}")
	}

	header.WriteString(":")

	return header.String()
}

func (te *toonEncoder) writeIndentedLine(writer io.Writer, depth int, content string) error {
	indent := strings.Repeat(" ", te.prefs.Indent*depth)
	return writeString(writer, indent+content+"\n")
}

const toonEscape = "\x1b"

func toonFormat(attr color.Attribute) string {
	return fmt.Sprintf("%s[%dm", toonEscape, attr)
}

// colourizeToonAndPrint applies syntax highlighting to TOON format output.
func colourizeToonAndPrint(toonBytes []byte, writer io.Writer) error {
	lines := bytes.Split(toonBytes, []byte("\n"))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		colourized := colourizeToonLine(line)
		if _, err := writer.Write(append(colourized, '\n')); err != nil {
			return err
		}
	}
	return nil
}

// colourizeToonLine applies colors to a single TOON line.
func colourizeToonLine(line []byte) []byte {
	lineStr := string(line)
	var result strings.Builder
	i := 0

	for i < len(lineStr) {
		c := lineStr[i]

		// Handle indentation (whitespace at start)
		if c == ' ' && i == 0 {
			result.WriteByte(c)
			i++
			continue
		}

		// Check for array header pattern: key[length]: or key[length]{fields}:
		if idx := strings.Index(lineStr[i:], "["); idx != -1 {
			// Check if this looks like an array header
			keyPart := lineStr[i : i+idx]
			if restIdx := strings.Index(lineStr[i+idx:], "]:"); restIdx != -1 {
				// This is an array header
				restStart := i + idx + restIdx + 2

				// Colourize key (cyan)
				result.WriteString(toonFormat(color.FgCyan))
				result.WriteString(keyPart)
				result.WriteString(toonFormat(color.Reset))

				// Colourize brackets and length (yellow)
				result.WriteString(toonFormat(color.FgHiYellow))
				result.WriteString(lineStr[i+idx : restStart])
				result.WriteString(toonFormat(color.Reset))

				// Check for fields in braces
				if restStart < len(lineStr) && lineStr[restStart] == '{' {
					braceEnd := strings.Index(lineStr[restStart:], "}")
					if braceEnd != -1 {
						braceEnd += restStart
						// Colourize braces and fields (magenta)
						result.WriteString(toonFormat(color.FgHiMagenta))
						result.WriteString(lineStr[restStart : braceEnd+1])
						result.WriteString(toonFormat(color.Reset))
						restStart = braceEnd + 1
					}
				}

				// Colourize colon (white/default)
				if restStart < len(lineStr) && lineStr[restStart] == ':' {
					result.WriteByte(':')
					restStart++
				}

				// Colourize inline values after colon
				if restStart < len(lineStr) {
					afterColon := strings.TrimSpace(lineStr[restStart:])
					if afterColon != "" {
						result.WriteByte(' ')
						colourizeToonValues(&result, afterColon, ",")
					}
				}

				return []byte(result.String())
			}
		}

		// Check for list item marker
		if c == '-' && (i == 0 || lineStr[i-1] == ' ') {
			// Check if this is a list item (followed by space or end of line)
			if i+1 >= len(lineStr) || lineStr[i+1] == ' ' {
				result.WriteString(toonFormat(color.FgHiYellow))
				result.WriteByte('-')
				result.WriteString(toonFormat(color.Reset))
				i++

				// Skip space after hyphen
				if i < len(lineStr) && lineStr[i] == ' ' {
					result.WriteByte(' ')
					i++
				}

				// Process rest of line
				if i < len(lineStr) {
					rest := lineStr[i:]
					// Check for key: value pattern
					if colonIdx := strings.Index(rest, ": "); colonIdx != -1 {
						key := rest[:colonIdx]
						value := rest[colonIdx+2:]

						// Colourize key (cyan)
						result.WriteString(toonFormat(color.FgCyan))
						result.WriteString(key)
						result.WriteString(toonFormat(color.Reset))
						result.WriteString(": ")

						// Colourize value
						colourizeToonValue(&result, value)
					} else if strings.HasSuffix(rest, ":") {
						// Just a key with colon (nested object)
						key := rest[:len(rest)-1]
						result.WriteString(toonFormat(color.FgCyan))
						result.WriteString(key)
						result.WriteString(toonFormat(color.Reset))
						result.WriteByte(':')
					} else {
						// Primitive value after hyphen
						colourizeToonValue(&result, rest)
					}
				}
				return []byte(result.String())
			}
		}

		// Check for key: value pattern
		if colonIdx := strings.Index(lineStr[i:], ": "); colonIdx != -1 {
			key := lineStr[i : i+colonIdx]
			value := lineStr[i+colonIdx+2:]

			// Colourize key (cyan)
			result.WriteString(toonFormat(color.FgCyan))
			result.WriteString(key)
			result.WriteString(toonFormat(color.Reset))
			result.WriteString(": ")

			// Colourize value
			colourizeToonValue(&result, value)
			return []byte(result.String())
		}

		// Check for key: (no value, nested object)
		if strings.HasSuffix(lineStr[i:], ":") {
			key := lineStr[i : len(lineStr)-1]
			result.WriteString(toonFormat(color.FgCyan))
			result.WriteString(key)
			result.WriteString(toonFormat(color.Reset))
			result.WriteByte(':')
			return []byte(result.String())
		}

		// Tabular row or primitive values (comma-separated)
		if strings.Contains(lineStr[i:], ",") {
			colourizeToonValues(&result, lineStr[i:], ",")
			return []byte(result.String())
		}

		// Fallback: colourize as primitive value
		colourizeToonValue(&result, lineStr[i:])
		return []byte(result.String())
	}

	return []byte(result.String())
}

// colourizeToonValue colourizes a single primitive value.
func colourizeToonValue(result *strings.Builder, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}

	// Quoted string (green)
	if strings.HasPrefix(value, "\"") {
		// Find end quote
		end := findEndQuote(value)
		if end == len(value)-1 {
			result.WriteString(toonFormat(color.FgGreen))
			result.WriteString(value)
			result.WriteString(toonFormat(color.Reset))
			return
		}
	}

	// Boolean (magenta)
	lower := strings.ToLower(value)
	if lower == "true" || lower == "false" {
		result.WriteString(toonFormat(color.FgHiMagenta))
		result.WriteString(value)
		result.WriteString(toonFormat(color.Reset))
		return
	}

	// Null (magenta)
	if lower == "null" {
		result.WriteString(toonFormat(color.FgHiMagenta))
		result.WriteString(value)
		result.WriteString(toonFormat(color.Reset))
		return
	}

	// Number (magenta)
	if isToonNumber(value) {
		result.WriteString(toonFormat(color.FgHiMagenta))
		result.WriteString(value)
		result.WriteString(toonFormat(color.Reset))
		return
	}

	// Default: string (no color)
	result.WriteString(value)
}

// colourizeToonValues colourizes comma-separated values.
func colourizeToonValues(result *strings.Builder, values string, delimiter string) {
	parts := splitToonValues(values, delimiter)
	for idx, part := range parts {
		if idx > 0 {
			result.WriteString(delimiter)
		}
		colourizeToonValue(result, part)
	}
}

// splitToonValues splits values respecting quoted strings.
func splitToonValues(s string, delimiter string) []string {
	var result []string
	var current strings.Builder
	inQuotes := false

	for i := 0; i < len(s); i++ {
		c := s[i]

		if c == '\\' && inQuotes && i+1 < len(s) {
			current.WriteByte(c)
			current.WriteByte(s[i+1])
			i++
			continue
		}

		if c == '"' {
			inQuotes = !inQuotes
			current.WriteByte(c)
			continue
		}

		if !inQuotes && strings.HasPrefix(s[i:], delimiter) {
			result = append(result, strings.TrimSpace(current.String()))
			current.Reset()
			i += len(delimiter) - 1
			continue
		}

		current.WriteByte(c)
	}

	if current.Len() > 0 || len(result) > 0 {
		result = append(result, strings.TrimSpace(current.String()))
	}

	return result
}

// findEndQuote finds the closing quote in a string.
func findEndQuote(s string) int {
	if len(s) < 2 || s[0] != '"' {
		return -1
	}
	for i := 1; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			continue
		}
		if s[i] == '"' {
			return i
		}
	}
	return -1
}

// isToonNumber checks if a string is a numeric literal.
func isToonNumber(s string) bool {
	if len(s) == 0 {
		return false
	}
	start := 0
	if s[0] == '-' {
		start = 1
	}
	if start >= len(s) {
		return false
	}

	hasDigit := false
	hasDot := false
	hasExp := false

	for i := start; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			hasDigit = true
			continue
		}
		if c == '.' && !hasDot && !hasExp {
			hasDot = true
			continue
		}
		if (c == 'e' || c == 'E') && !hasExp {
			hasExp = true
			if i+1 < len(s) && (s[i+1] == '+' || s[i+1] == '-') {
				i++
			}
			continue
		}
		return false
	}

	return hasDigit
}

