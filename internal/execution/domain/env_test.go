package domain

import "testing"

func TestValidateRuntimeBaseURLAcceptsOnlyAnHTTPOrigin(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{name: "https origin", in: " https://qa.example.test/ ", want: "https://qa.example.test", ok: true},
		{name: "http origin", in: "http://sut.internal", want: "http://sut.internal", ok: true},
		{name: "missing scheme", in: "qa.example.test", ok: false},
		{name: "credentials", in: "https://user:" + "pass@qa.example.test", ok: false},
		{name: "path", in: "https://qa.example.test/api", ok: false},
		{name: "query", in: "https://qa.example.test?debug=1", ok: false},
		{name: "fragment", in: "https://qa.example.test#section", ok: false},
		{name: "file scheme", in: "file:///etc/passwd", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateRuntimeBaseURL(tt.in)
			if tt.ok {
				if err != nil || got != tt.want {
					t.Fatalf("ValidateRuntimeBaseURL(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateRuntimeBaseURL(%q) unexpectedly accepted %q", tt.in, got)
			}
		})
	}
}
