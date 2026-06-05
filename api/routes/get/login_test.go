package get

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoginPostMessageOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		oauthURI string
		want     string
	}{
		{
			name:     "api subdomain",
			oauthURI: "https://api.potat.industries",
			want:     "https://potat.industries",
		},
		{
			name:     "api subdomain with trailing slash",
			oauthURI: "https://api.potat.industries/",
			want:     "https://potat.industries",
		},
		{
			name:     "api subdomain with port",
			oauthURI: "https://api.potat.industries:8443/login",
			want:     "https://potat.industries:8443",
		},
		{
			name:     "localhost",
			oauthURI: "http://localhost:8080",
			want:     "http://localhost:8080",
		},
		{
			name:     "fallback",
			oauthURI: "api.potat.industries/",
			want:     "potat.industries",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, loginPostMessageOrigin(test.oauthURI))
		})
	}
}
