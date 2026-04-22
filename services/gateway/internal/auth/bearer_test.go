package auth

import "testing"

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantToken string
		wantErr   error
	}{
		{
			name:      "success",
			header:    "Bearer token-123",
			wantToken: "token-123",
		},
		{
			name:      "success with extra spaces",
			header:    "  Bearer    token-123   ",
			wantToken: "token-123",
		},
		{
			name:    "missing header",
			header:  "",
			wantErr: ErrMissingAuthHeader,
		},
		{
			name:    "invalid scheme",
			header:  "Basic token-123",
			wantErr: ErrInvalidAuthHeaderFormat,
		},
		{
			name:    "missing token",
			header:  "Bearer   ",
			wantErr: ErrInvalidAuthHeaderFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractBearerToken(tt.header)
			if err != tt.wantErr {
				t.Fatalf("expected err %v, got %v", tt.wantErr, err)
			}
			if got != tt.wantToken {
				t.Fatalf("expected token %q, got %q", tt.wantToken, got)
			}
		})
	}
}
