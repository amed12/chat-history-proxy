package security

import "testing"

func TestMaskDatabaseURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "postgres://postgres:my-secret-password@localhost:5432/chat_history?sslmode=disable",
			expected: "postgres://postgres:xxx@localhost:5432/chat_history?sslmode=disable",
		},
		{
			input:    "postgres://user@db:5432/db",
			expected: "postgres://user@db:5432/db",
		},
		{
			input:    "host=localhost port=5432 user=postgres password=secret_pass dbname=test",
			expected: "host=localhost port=5432 user=postgres password=xxx dbname=test",
		},
		{
			input:    "password='secret' user=postgres",
			expected: "password='xxx' user=postgres",
		},
		{
			input:    `password="secret" user=postgres`,
			expected: `password="xxx" user=postgres`,
		},
		{
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		got := MaskDatabaseURL(tt.input)
		if got != tt.expected {
			t.Errorf("MaskDatabaseURL(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}
