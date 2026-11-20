package tools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateInput(t *testing.T) {
	findingSchema := SchemaFor(FindingInput{})
	readSchema := SchemaFor(ReadLinesInput{})
	cases := []struct {
		name   string
		schema json.RawMessage
		input  string
		errMsg string
	}{
		{"valid read", readSchema, `{"path":"a.go","start":1,"end":5}`, ""},
		{"missing required", readSchema, `{"path":"a.go","start":1}`, `missing required field "end"`},
		{"wrong type", readSchema, `{"path":3,"start":1,"end":5}`, "input.path must be a string"},
		{"fractional integer", readSchema, `{"path":"a","start":1.5,"end":5}`, "must be an integer"},
		{"unknown field", readSchema, `{"path":"a","start":1,"end":5,"x":1}`, `unknown field "x"`},
		{"truncated json", readSchema, `{"path":"a","sta`, "not valid JSON"},
		{"not an object", readSchema, `[1,2]`, "must be an object"},
		{"bad enum", findingSchema, `{"severity":"huge","category":"tests","file":"a","line":1,"title":"t","body":"b","confidence":0.9}`, "must be one of"},
		{"valid finding", findingSchema, `{"severity":"warning","category":"tests","file":"a","line":1,"title":"t","body":"b","confidence":1}`, ""},
		{"null input on empty schema", SchemaFor(RiskRankingInput{}), `null`, ""},
		{"extra field on empty schema", SchemaFor(RiskRankingInput{}), `{"x":1}`, "unknown field"},
	}
	for _, testCase := range cases {
		err := ValidateInput(testCase.schema, json.RawMessage(testCase.input))
		if testCase.errMsg == "" {
			assert.NoError(t, err, testCase.name)
			continue
		}
		assert.ErrorContains(t, err, testCase.errMsg, testCase.name)
	}
}

func TestValidateInputArrayItems(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"string"}}}}`)
	assert.NoError(t, ValidateInput(schema, json.RawMessage(`{"items":["a"]}`)))
	assert.ErrorContains(t, ValidateInput(schema, json.RawMessage(`{"items":["a",2]}`)), "input.items[1] must be a string")
	assert.ErrorContains(t, ValidateInput(schema, json.RawMessage(`{"items":"a"}`)), "must be an array")
}

func TestCallValidatesBeforeRunning(t *testing.T) {
	toolbox, _ := newFixture(t)
	_, err := toolbox.Call(ToolReportFinding, json.RawMessage(`{"severity":"critical","file":"calc/calc.go"}`))
	assert.ErrorContains(t, err, "missing required field")
	assert.Empty(t, toolbox.Findings())
}
