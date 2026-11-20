package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAuthFailure(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"Invalid API key · Please run /login", true},
		{"Not logged in", true},
		{`API Error: 401 {"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`, true},
		{"API Error: 401 Unauthorized", true},
		{"request failed with status 401", true},
		{"HTTP 401: token rejected", true},
		{"anthropic API rejected the credentials (401)", true},
		{"OAuth token has expired", true},
		{"panic at handler.go line 401", false},
		{"reviewed 401 files", false},
		{"see pkg/server.go:401 for details", false},
		{"", false},
	}
	for _, testCase := range cases {
		assert.Equal(t, testCase.want, isAuthFailure(testCase.text), testCase.text)
	}
}
