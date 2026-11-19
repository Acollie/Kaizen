package review

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const modifiedDiff = `diff --git a/pkg/app/run.go b/pkg/app/run.go
index 1111111..2222222 100644
--- a/pkg/app/run.go
+++ b/pkg/app/run.go
@@ -10,4 +10,5 @@ func Run() error {
 	setup()
-	old()
+	first()
+	second()
 	return nil
 }
`

const multiHunkDiff = `diff --git a/main.py b/main.py
index 1111111..2222222 100644
--- a/main.py
+++ b/main.py
@@ -1,3 +1,3 @@
 import os
-import sys
+import json

@@ -20,2 +20,3 @@ def handler(event):
     value = event["x"]
+    log(value)
     return value
`

const addedDiff = `diff --git a/new.go b/new.go
new file mode 100644
index 0000000..3333333
--- /dev/null
+++ b/new.go
@@ -0,0 +1,3 @@
+package app
+
+func New() {}
`

const deletedDiff = `diff --git a/gone.kt b/gone.kt
deleted file mode 100644
index 4444444..0000000
--- a/gone.kt
+++ /dev/null
@@ -1,2 +0,0 @@
-fun gone() {
-}
`

const renamedDiff = `diff --git a/old/name.swift b/new/name.swift
similarity index 90%
rename from old/name.swift
rename to new/name.swift
index 5555555..6666666 100644
--- a/old/name.swift
+++ b/new/name.swift
@@ -1,2 +1,2 @@
-func a() {}
+func b() {}
 // end
`

const pureRenameDiff = `diff --git a/a.kts b/b.kts
similarity index 100%
rename from a.kts
rename to b.kts
`

const binaryDiff = `diff --git a/logo.png b/logo.png
new file mode 100644
index 0000000..7777777
Binary files /dev/null and b/logo.png differ
`

const noNewlineDiff = `diff --git a/x.txt b/x.txt
index 1111111..2222222 100644
--- a/x.txt
+++ b/x.txt
@@ -1,2 +1,2 @@
 keep
-last
\ No newline at end of file
+last line
\ No newline at end of file
`

const trickyContentDiff = `diff --git a/notes.txt b/notes.txt
index 1111111..2222222 100644
--- a/notes.txt
+++ b/notes.txt
@@ -1,2 +1,2 @@
--- this removed line looks like a header
+++ this added line looks like a header
 diff stays
`

func TestParseUnifiedDiffFileSummaries(t *testing.T) {
	testCases := []struct {
		name      string
		diff      string
		path      string
		oldPath   string
		status    FileStatus
		language  string
		additions int
		deletions int
		binary    bool
		hunkCount int
	}{
		{"modified", modifiedDiff, "pkg/app/run.go", "pkg/app/run.go", FileModified, "Go", 2, 1, false, 1},
		{"multiple hunks", multiHunkDiff, "main.py", "main.py", FileModified, "Python", 2, 1, false, 2},
		{"added", addedDiff, "new.go", "new.go", FileAdded, "Go", 3, 0, false, 1},
		{"deleted", deletedDiff, "gone.kt", "gone.kt", FileDeleted, "Kotlin", 0, 2, false, 1},
		{"renamed with edits", renamedDiff, "new/name.swift", "old/name.swift", FileRenamed, "Swift", 1, 1, false, 1},
		{"pure rename", pureRenameDiff, "b.kts", "a.kts", FileRenamed, "Kotlin", 0, 0, false, 0},
		{"binary", binaryDiff, "logo.png", "logo.png", FileAdded, "", 0, 0, true, 0},
		{"no newline marker", noNewlineDiff, "x.txt", "x.txt", FileModified, "", 1, 1, false, 1},
		{"content resembling headers", trickyContentDiff, "notes.txt", "notes.txt", FileModified, "", 1, 1, false, 1},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			files, err := ParseUnifiedDiff(testCase.diff)
			require.NoError(t, err)
			require.Len(t, files, 1)

			file := files[0]
			assert.Equal(t, testCase.path, file.Path)
			assert.Equal(t, testCase.oldPath, file.OldPath)
			assert.Equal(t, testCase.status, file.Status)
			assert.Equal(t, testCase.language, file.Language)
			assert.Equal(t, testCase.additions, file.Additions)
			assert.Equal(t, testCase.deletions, file.Deletions)
			assert.Equal(t, testCase.binary, file.Binary)
			assert.Len(t, file.Hunks, testCase.hunkCount)
		})
	}
}

func TestParseUnifiedDiffLineNumbers(t *testing.T) {
	files, err := ParseUnifiedDiff(modifiedDiff)
	require.NoError(t, err)
	hunk := files[0].Hunks[0]

	assert.Equal(t, 10, hunk.OldStart)
	assert.Equal(t, 4, hunk.OldCount)
	assert.Equal(t, 10, hunk.NewStart)
	assert.Equal(t, 5, hunk.NewCount)
	assert.Equal(t, "func Run() error {", hunk.Header)

	expected := []DiffLine{
		{Kind: LineContext, OldLine: 10, NewLine: 10, Text: "\tsetup()"},
		{Kind: LineRemoved, OldLine: 11, NewLine: 0, Text: "\told()"},
		{Kind: LineAdded, OldLine: 0, NewLine: 11, Text: "\tfirst()"},
		{Kind: LineAdded, OldLine: 0, NewLine: 12, Text: "\tsecond()"},
		{Kind: LineContext, OldLine: 12, NewLine: 13, Text: "\treturn nil"},
		{Kind: LineContext, OldLine: 13, NewLine: 14, Text: "}"},
	}
	assert.Equal(t, expected, hunk.Lines)
}

func TestParseUnifiedDiffMultipleHunks(t *testing.T) {
	files, err := ParseUnifiedDiff(multiHunkDiff)
	require.NoError(t, err)
	hunks := files[0].Hunks

	// The blank context line in the first hunk keeps its numbering
	assert.Equal(t, DiffLine{Kind: LineContext, OldLine: 3, NewLine: 3, Text: ""}, hunks[0].Lines[3])

	assert.Equal(t, "def handler(event):", hunks[1].Header)
	assert.Equal(t, DiffLine{Kind: LineAdded, NewLine: 21, Text: "    log(value)"}, hunks[1].Lines[1])
	assert.Equal(t, DiffLine{Kind: LineContext, OldLine: 21, NewLine: 22, Text: "    return value"}, hunks[1].Lines[2])
}

func TestParseUnifiedDiffAddedAndDeletedLineNumbers(t *testing.T) {
	added, err := ParseUnifiedDiff(addedDiff)
	require.NoError(t, err)
	assert.Equal(t, DiffLine{Kind: LineAdded, NewLine: 3, Text: "func New() {}"}, added[0].Hunks[0].Lines[2])

	deleted, err := ParseUnifiedDiff(deletedDiff)
	require.NoError(t, err)
	assert.Equal(t, DiffLine{Kind: LineRemoved, OldLine: 2, Text: "}"}, deleted[0].Hunks[0].Lines[1])
}

func TestParseUnifiedDiffNoNewlineMarkers(t *testing.T) {
	files, err := ParseUnifiedDiff(noNewlineDiff)
	require.NoError(t, err)
	lines := files[0].Hunks[0].Lines

	require.Len(t, lines, 3)
	assert.Equal(t, LineRemoved, lines[1].Kind)
	assert.Equal(t, "last", lines[1].Text)
	assert.Equal(t, DiffLine{Kind: LineAdded, NewLine: 2, Text: "last line"}, lines[2])
}

func TestParseUnifiedDiffHeaderLookalikes(t *testing.T) {
	files, err := ParseUnifiedDiff(trickyContentDiff)
	require.NoError(t, err)
	lines := files[0].Hunks[0].Lines

	require.Len(t, lines, 3)
	assert.Equal(t, "-- this removed line looks like a header", lines[0].Text)
	assert.Equal(t, "++ this added line looks like a header", lines[1].Text)
	assert.Equal(t, "notes.txt", files[0].Path)
}

func TestParseUnifiedDiffMultipleFiles(t *testing.T) {
	combined := modifiedDiff + addedDiff + binaryDiff + deletedDiff
	files, err := ParseUnifiedDiff(combined)
	require.NoError(t, err)
	require.Len(t, files, 4)

	paths := []string{files[0].Path, files[1].Path, files[2].Path, files[3].Path}
	assert.Equal(t, []string{"pkg/app/run.go", "new.go", "logo.png", "gone.kt"}, paths)
}

func TestParseUnifiedDiffQuotedAndSpacedPaths(t *testing.T) {
	diff := "diff --git \"a/dir/sp\\303\\244ce.go\" \"b/dir/sp\\303\\244ce.go\"\n" +
		"index 1..2 100644\n" +
		"--- \"a/dir/sp\\303\\244ce.go\"\n" +
		"+++ \"b/dir/sp\\303\\244ce.go\"\n" +
		"@@ -1 +1 @@\n" +
		"-a\n" +
		"+b\n" +
		"diff --git a/with space.go b/with space.go\n" +
		"new file mode 100644\n" +
		"Binary files /dev/null and b/with space.go differ\n"

	files, err := ParseUnifiedDiff(diff)
	require.NoError(t, err)
	require.Len(t, files, 2)
	assert.Equal(t, "dir/späce.go", files[0].Path)
	assert.Equal(t, 1, files[0].Hunks[0].OldCount)
	assert.Equal(t, "with space.go", files[1].Path)
	assert.True(t, files[1].Binary)
}

func TestParseUnifiedDiffEmptyAndMalformed(t *testing.T) {
	files, err := ParseUnifiedDiff("")
	require.NoError(t, err)
	assert.Empty(t, files)

	_, err = ParseUnifiedDiff("diff --git a/x b/x\n@@ nonsense @@\n")
	assert.Error(t, err)
}

func TestLanguageForPath(t *testing.T) {
	testCases := map[string]string{
		"main.go":          "Go",
		"tool.py":          "Python",
		"App.kt":           "Kotlin",
		"build.gradle.kts": "Kotlin",
		"View.swift":       "Swift",
		"README.md":        "",
		"Makefile":         "",
	}
	for path, expected := range testCases {
		assert.Equal(t, expected, LanguageForPath(path), path)
	}
}
