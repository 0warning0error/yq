//go:build !yq_notoon

package yqlib

import (
	"bufio"
	"bytes"
	"fmt"
	"testing"

	"github.com/mikefarah/yq/v4/test"
)

var toonFormatScenarios = []formatScenario{
	// Primitives
	{
		description:  "Decode string",
		input:        `name: hello`,
		expected:     "name: hello\n",
		scenarioType: "decode",
	},
	{
		description:  "Decode quoted string",
		input:        `message: "hello world"`,
		expected:     "message: hello world\n",
		scenarioType: "decode",
	},
	{
		description:  "Decode number",
		input:        `count: 42`,
		expected:     "count: 42\n",
		scenarioType: "decode",
	},
	{
		description:  "Decode float",
		input:        `pi: 3.14`,
		expected:     "pi: 3.14\n",
		scenarioType: "decode",
	},
	{
		description:  "Decode boolean true",
		input:        `enabled: true`,
		expected:     "enabled: true\n",
		scenarioType: "decode",
	},
	{
		description:  "Decode boolean false",
		input:        `enabled: false`,
		expected:     "enabled: false\n",
		scenarioType: "decode",
	},
	{
		description:  "Decode null",
		input:        `value: null`,
		expected:     "value: null\n",
		scenarioType: "decode",
	},

	// Objects
	{
		description:  "Decode nested object",
		input:        "user:\n  name: Ada\n  id: 123",
		expected:     "user:\n  name: Ada\n  id: 123\n",
		scenarioType: "decode",
	},
	{
		description:  "Decode empty object",
		input:        `config:`,
		expected:     "config: {}\n",
		scenarioType: "decode",
	},
	{
		description:  "Decode multiple fields",
		input:        "name: app\nversion: 1\nenabled: true",
		expected:     "name: app\nversion: 1\nenabled: true\n",
		scenarioType: "decode",
	},

	// Arrays - inline primitive arrays
	{
		description:  "Decode inline primitive array",
		input:        `tags[3]: admin,ops,dev`,
		expected:     "tags:\n  - admin\n  - ops\n  - dev\n",
		scenarioType: "decode",
	},
	{
		description:  "Decode empty array",
		input:        `items[0]:`,
		expected:     "items: []\n",
		scenarioType: "decode",
	},
	{
		description:  "Decode number array",
		input:        `numbers[4]: 1,2,3,4`,
		expected:     "numbers:\n  - 1\n  - 2\n  - 3\n  - 4\n",
		scenarioType: "decode",
	},

	// Arrays - tabular format
	{
		description:  "Decode tabular array",
		input:        "items[2]{sku,qty,price}:\n  A1,2,9.99\n  B2,1,14.5",
		expected:     "items:\n  - sku: A1\n    qty: 2\n    price: 9.99\n  - sku: B2\n    qty: 1\n    price: 14.5\n",
		scenarioType: "decode",
	},

	// Arrays - list format
	{
		description:  "Decode list array with primitives",
		input:        "items[3]:\n  - one\n  - two\n  - three",
		expected:     "items:\n  - one\n  - two\n  - three\n",
		scenarioType: "decode",
	},
	{
		description:  "Decode list array with objects",
		input:        "users[2]:\n  - name: Ada\n    id: 1\n  - name: Bob\n    id: 2",
		expected:     "users:\n  - name: Ada\n    id: 1\n  - name: Bob\n    id: 2\n",
		scenarioType: "decode",
	},
	{
		description:  "Decode mixed array",
		input:        "mixed[3]:\n  - 1\n  - text\n  - true",
		expected:     "mixed:\n  - 1\n  - text\n  - true\n",
		scenarioType: "decode",
	},

	// Roundtrip tests
	{
		description:  "Roundtrip simple object",
		input:        `name: hello`,
		expected:     "name: hello\n",
		scenarioType: "roundtrip",
	},
	{
		description:  "Roundtrip nested object",
		input:        "user:\n  name: Ada\n  id: 123",
		expected:     "user:\n  name: Ada\n  id: 123\n",
		scenarioType: "roundtrip",
	},
	{
		description:  "Roundtrip inline array",
		input:        `tags[3]: admin,ops,dev`,
		expected:     "tags[3]: admin,ops,dev\n",
		scenarioType: "roundtrip",
	},
	{
		description:  "Roundtrip tabular array",
		input:        "items[2]{sku,qty,price}:\n  A1,2,9.99\n  B2,1,14.5",
		expected:     "items[2]{sku,qty,price}:\n  A1,2,9.99\n  B2,1,14.5\n",
		scenarioType: "roundtrip",
	},

	// Complex scenarios
	{
		description:  "Decode complex nested structure",
		input:        "config:\n  database:\n    host: localhost\n    port: 5432\n  cache:\n    enabled: true\n    ttl: 3600",
		expected:     "config:\n  database:\n    host: localhost\n    port: 5432\n  cache:\n    enabled: true\n    ttl: 3600\n",
		scenarioType: "decode",
	},
	{
		description:  "Decode object with nested array",
		input:        "user:\n  name: Ada\n  roles[2]: admin,user",
		expected:     "user:\n  name: Ada\n  roles:\n    - admin\n    - user\n",
		scenarioType: "decode",
	},
	{
		description:  "Decode quoted key",
		input:        `"key-with-hyphen": value`,
		expected:     "key-with-hyphen: value\n",
		scenarioType: "decode",
	},

	// Edge cases
	{
		description:  "Decode string with spaces",
		input:        `message: "hello world"`,
		expected:     "message: hello world\n",
		scenarioType: "decode",
	},
	{
		description:  "Decode string with escape",
		input:        `text: "line1\nline2"`,
		expected:     "text: |-\n  line1\n  line2\n",
		scenarioType: "decode",
		skipDoc:      true,
	},
	{
		description:  "Decode negative number",
		input:        `count: -42`,
		expected:     "count: -42\n",
		scenarioType: "decode",
	},
	{
		description:  "Decode scientific notation",
		input:        `value: 1e-3`,
		expected:     "value: 1e-3\n",
		scenarioType: "decode",
	},
}

func testToonScenario(t *testing.T, s formatScenario) {
	// Disable colors for testing to avoid ANSI escape codes in comparisons
	toonPrefs := ConfiguredToonPreferences
	toonPrefs.ColorsEnabled = false

	switch s.scenarioType {
	case "decode":
		result := mustProcessFormatScenario(s, NewToonDecoder(), NewYamlEncoder(ConfiguredYamlPreferences))
		test.AssertResultWithContext(t, s.expected, result, s.description)
	case "roundtrip":
		test.AssertResultWithContext(t, s.expected, mustProcessFormatScenario(s, NewToonDecoder(), NewToonEncoder(toonPrefs)), s.description)
	}
}

func documentToonScenario(_ *testing.T, w *bufio.Writer, i interface{}) {
	s := i.(formatScenario)

	if s.skipDoc {
		return
	}
	switch s.scenarioType {
	case "", "decode":
		documentToonDecodeScenario(w, s)
	case "roundtrip":
		documentToonRoundTripScenario(w, s)
	default:
		panic(fmt.Sprintf("unhandled scenario type %q", s.scenarioType))
	}
}

func documentToonDecodeScenario(w *bufio.Writer, s formatScenario) {
	writeOrPanic(w, fmt.Sprintf("## %v\n", s.description))

	if s.subdescription != "" {
		writeOrPanic(w, s.subdescription)
		writeOrPanic(w, "\n\n")
	}

	writeOrPanic(w, "Given a sample.toon file of:\n")
	writeOrPanic(w, fmt.Sprintf("```toon\n%v\n```\n", s.input))

	writeOrPanic(w, "then\n")
	expression := s.expression
	if s.expression != "" {
		expression = fmt.Sprintf(" '%v'", s.expression)
	}
	writeOrPanic(w, fmt.Sprintf("```bash\nyq -oy%v sample.toon\n```\n", expression))
	writeOrPanic(w, "will output\n")

	writeOrPanic(w, fmt.Sprintf("```yaml\n%v```\n\n", mustProcessFormatScenario(s, NewToonDecoder(), NewYamlEncoder(ConfiguredYamlPreferences))))
}

func documentToonRoundTripScenario(w *bufio.Writer, s formatScenario) {
	writeOrPanic(w, fmt.Sprintf("## %v\n", s.description))

	if s.subdescription != "" {
		writeOrPanic(w, s.subdescription)
		writeOrPanic(w, "\n\n")
	}

	writeOrPanic(w, "Given a sample.toon file of:\n")
	writeOrPanic(w, fmt.Sprintf("```toon\n%v\n```\n", s.input))

	writeOrPanic(w, "then\n")
	expression := s.expression
	if s.expression != "" {
		expression = fmt.Sprintf(" '%v'", s.expression)
	}
	writeOrPanic(w, fmt.Sprintf("```bash\nyq -o toon%v sample.toon\n```\n", expression))
	writeOrPanic(w, "will output\n")

	// Disable colors for documentation output
	toonPrefs := ConfiguredToonPreferences
	toonPrefs.ColorsEnabled = false
	writeOrPanic(w, fmt.Sprintf("```toon\n%v```\n\n", mustProcessFormatScenario(s, NewToonDecoder(), NewToonEncoder(toonPrefs))))
}

func TestToonEncoderPrintDocumentSeparator(t *testing.T) {
	encoder := NewToonEncoder(ConfiguredToonPreferences)
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)

	err := encoder.PrintDocumentSeparator(writer)
	writer.Flush()

	test.AssertResult(t, nil, err)
	test.AssertResult(t, "", buf.String())
}

func TestToonEncoderPrintLeadingContent(t *testing.T) {
	encoder := NewToonEncoder(ConfiguredToonPreferences)
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)

	err := encoder.PrintLeadingContent(writer, "some content")
	writer.Flush()

	test.AssertResult(t, nil, err)
	test.AssertResult(t, "", buf.String())
}

func TestToonEncoderCanHandleAliases(t *testing.T) {
	encoder := NewToonEncoder(ConfiguredToonPreferences)
	test.AssertResult(t, false, encoder.CanHandleAliases())
}

func TestToonFormatScenarios(t *testing.T) {
	for _, tt := range toonFormatScenarios {
		testToonScenario(t, tt)
	}
	genericScenarios := make([]interface{}, len(toonFormatScenarios))
	for i, s := range toonFormatScenarios {
		genericScenarios[i] = s
	}
	documentScenarios(t, "usage", "toon", genericScenarios, documentToonScenario)
}
