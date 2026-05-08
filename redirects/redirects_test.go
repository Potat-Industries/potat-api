package redirects

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedirects__CheckProtocolFormatAfterProtocolReformat(t *testing.T) {
	redirector := redirects{}

	tests := []struct {
		input    string
		expected string
	}{
		{"https://google.com", "https://google.com"},
		{"http://google.com", "https://google.com"},
		{"//google.com", "https://google.com"},
		{"google.com", "https://google.com"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			cleanedURL := redirector.cleanRedirectProtocolSoLinksActuallyWork(tc.input)
			assert.Truef(
				t, strings.HasPrefix(cleanedURL, "https://"), "Expected cleaned URL to start with 'https://', got %q", cleanedURL,
			)
			assert.Equal(t, tc.expected, cleanedURL)
		})
	}
}
