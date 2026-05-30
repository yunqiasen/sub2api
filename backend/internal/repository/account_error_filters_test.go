package repository

import "testing"

func TestClassifyAccountErrorMessage(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    accountErrorBucket
	}{
		{
			name:    "token invalidated is invalid credentials",
			message: "token_invalidated: Your authentication token has been invalidated. Please try signing in again.",
			want:    accountErrorBucketAuthInvalid,
		},
		{
			name:    "revoked token is invalid credentials",
			message: "Token revoked (401)",
			want:    accountErrorBucketAuthInvalid,
		},
		{
			name:    "eof is network",
			message: `Post "https://chatgpt.com/backend-api/codex/responses": EOF`,
			want:    accountErrorBucketNetwork,
		},
		{
			name:    "proxy timeout is network",
			message: "proxyconnect tcp: i/o timeout",
			want:    accountErrorBucketNetwork,
		},
		{
			name:    "cloudflare challenge is cf",
			message: "OpenAI 403 temporary cooldown: Access forbidden Cloudflare challenge",
			want:    accountErrorBucketCF,
		},
		{
			name:    "unknown model error is other",
			message: "model not available for this account",
			want:    accountErrorBucketOther,
		},
		{
			name:    "empty error is other",
			message: "",
			want:    accountErrorBucketOther,
		},
		{
			name:    "invalid credential wins over cf when both appear",
			message: "403 token_invalidated",
			want:    accountErrorBucketAuthInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyAccountErrorMessage(tt.message); got != tt.want {
				t.Fatalf("classifyAccountErrorMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}
