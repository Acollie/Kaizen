package review

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMarkGeneratedFiles(t *testing.T) {
	added := func(text string) []Hunk {
		return []Hunk{{Lines: []DiffLine{{Kind: LineAdded, NewLine: 1, Text: text}}}}
	}
	files := []FileDiff{
		{Path: "web/package-lock.json", Hunks: added(`"name": "x"`)},
		{Path: "static/assets/app.js", Hunks: added(strings.Repeat("a=1;", 200))},
		{Path: "vendor/lib.min.css", Hunks: added("a{}")},
		{Path: "pkg/app/app.go", Hunks: added("func main() {}")},
	}
	generated := markGeneratedFiles(files)

	assert.Equal(t, map[string]bool{"web/package-lock.json": true, "static/assets/app.js": true, "vendor/lib.min.css": true}, generated)
	for _, file := range files[:3] {
		assert.True(t, file.Generated, file.Path)
		assert.Nil(t, file.Hunks, file.Path)
		assert.NotEmpty(t, file.RiskReasons, file.Path)
	}
	assert.False(t, files[3].Generated)
	assert.Len(t, files[3].Hunks, 1)
}
