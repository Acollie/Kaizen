package reviewui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// maxStateBodyBytes caps a saved review state document (triage, notes, comments, viewed)
const maxStateBodyBytes = 2 << 20

// errStateNotObject rejects a state document that is not a JSON object
var errStateNotObject = errors.New("state must be a JSON object")

// handleGetState returns the reviewer's saved work for this PR head, or 404 when
// nothing has been saved yet (or persistence is not configured)
func (server *Server) handleGetState(writer http.ResponseWriter, request *http.Request) {
	if !server.checkWriteGuards(writer, request, false) {
		return
	}
	if server.options.StatePath == "" {
		writeError(writer, http.StatusNotFound, "state persistence is not configured for this session")
		return
	}
	server.stateMutex.Lock()
	content, err := os.ReadFile(server.options.StatePath)
	server.stateMutex.Unlock()
	if errors.Is(err, os.ErrNotExist) {
		writeError(writer, http.StatusNotFound, "no saved state")
		return
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "could not read saved state")
		return
	}
	if validateStateDocument(content) != nil {
		writeError(writer, http.StatusNotFound, "saved state is unreadable")
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(content)
}

// handlePutState saves the reviewer's work. Like POST /api/post it needs the token in
// the header, an allowed Origin and a JSON body, so another site cannot overwrite it.
func (server *Server) handlePutState(writer http.ResponseWriter, request *http.Request) {
	if !server.checkWriteGuards(writer, request, true) {
		return
	}
	if server.options.StatePath == "" {
		writeError(writer, http.StatusNotImplemented, "state persistence is not configured for this session")
		return
	}
	content, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, maxStateBodyBytes))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(writer, http.StatusRequestEntityTooLarge, fmt.Sprintf("state is larger than %d bytes", maxStateBodyBytes))
		return
	}
	if err != nil {
		writeError(writer, http.StatusBadRequest, "could not read body")
		return
	}
	if err := validateStateDocument(content); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	server.stateMutex.Lock()
	err = writeFileAtomic(server.options.StatePath, content)
	server.stateMutex.Unlock()
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "could not save state")
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

// validateStateDocument accepts exactly one JSON object
func validateStateDocument(content []byte) error {
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
		return errStateNotObject
	}
	return nil
}

// writeFileAtomic writes to a temp file in the target's directory and renames it over
// the target, so a crash mid-write never leaves a truncated state file
func writeFileAtomic(path string, content []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(directory, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }() // a no-op once renamed

	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

// StatePathFor returns the state file that sits next to a review cache file:
// .kaizen/reviews/pr-<n>-<head12>.json -> .kaizen/reviews/pr-<n>-<head12>.state.json.
// It is keyed by the PR head like the cache, so a new push starts a fresh review.
func StatePathFor(reviewCachePath string) string {
	return strings.TrimSuffix(reviewCachePath, filepath.Ext(reviewCachePath)) + ".state.json"
}
