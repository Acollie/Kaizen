package check

import (
	"testing"
)

func TestParseEmptyDiff(t *testing.T) {
	result, err := ParseDiffOutput("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestParseSingleHunk(t *testing.T) {
	diff := `diff --git a/pkg/test.go b/pkg/test.go
index abc123..def456 100644
--- a/pkg/test.go
+++ b/pkg/test.go
@@ -10,3 +10,5 @@
 line 10
 line 11
+added 1
+added 2`

	hunks, err := ParseDiffOutput(diff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	if hunks[0].FilePath != "pkg/test.go" {
		t.Fatalf("expected pkg/test.go, got %s", hunks[0].FilePath)
	}
	if hunks[0].NewStart != 10 {
		t.Fatalf("expected start line 10, got %d", hunks[0].NewStart)
	}
	if hunks[0].NewCount != 5 {
		t.Fatalf("expected count 5, got %d", hunks[0].NewCount)
	}
}

func TestParseMultipleHunks(t *testing.T) {
	diff := `diff --git a/pkg/test.go b/pkg/test.go
@@ -10 +10,2 @@
 line 10
+added
@@ -20 +22,1 @@
 line 20
+changed`

	hunks, err := ParseDiffOutput(diff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(hunks))
	}
}

func TestParseDeletionOnlyHunk(t *testing.T) {
	diff := `diff --git a/pkg/test.go b/pkg/test.go
@@ -10,2 +10,0 @@
-deleted 1
-deleted 2`

	hunks, err := ParseDiffOutput(diff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Deletion-only hunks (newCount=0) should be skipped
	if len(hunks) != 0 {
		t.Fatalf("expected 0 hunks (deletion only), got %d", len(hunks))
	}
}

func TestParseMissingCount(t *testing.T) {
	diff := `diff --git a/pkg/test.go b/pkg/test.go
@@ -10 +10 @@
 line 10
+added`

	hunks, err := ParseDiffOutput(diff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	// Missing count should default to 1
	if hunks[0].NewCount != 1 {
		t.Fatalf("expected count 1 (default), got %d", hunks[0].NewCount)
	}
}

func TestParseMultipleFiles(t *testing.T) {
	diff := `diff --git a/pkg/test1.go b/pkg/test1.go
@@ -10 +10,1 @@
+added
diff --git a/pkg/test2.go b/pkg/test2.go
@@ -20 +20,1 @@
+added`

	hunks, err := ParseDiffOutput(diff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(hunks))
	}
	if hunks[0].FilePath != "pkg/test1.go" {
		t.Fatalf("expected pkg/test1.go, got %s", hunks[0].FilePath)
	}
	if hunks[1].FilePath != "pkg/test2.go" {
		t.Fatalf("expected pkg/test2.go, got %s", hunks[1].FilePath)
	}
}

func TestParseBinaryFiles(t *testing.T) {
	diff := `diff --git a/image.png b/image.png
Binary files a/image.png and b/image.png differ
diff --git a/pkg/test.go b/pkg/test.go
@@ -10 +10,1 @@
+added`

	hunks, err := ParseDiffOutput(diff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Binary file should be skipped, only test.go hunk should remain
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	if hunks[0].FilePath != "pkg/test.go" {
		t.Fatalf("expected pkg/test.go, got %s", hunks[0].FilePath)
	}
}

func TestParseRenamePrefersBPath(t *testing.T) {
	diff := `diff --git a/old_name.go b/new_name.go
@@ -10 +10,1 @@
+added`

	hunks, err := ParseDiffOutput(diff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	// Should use b/ side (new name)
	if hunks[0].FilePath != "new_name.go" {
		t.Fatalf("expected new_name.go, got %s", hunks[0].FilePath)
	}
}
