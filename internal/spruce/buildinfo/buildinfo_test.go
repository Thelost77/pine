package buildinfo

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "empty", version: "", want: "dev"},
		{name: "local build", version: "(devel)", want: "dev"},
		{name: "tagged build", version: "v0.1.0", want: "v0.1.0"},
		{name: "trim whitespace", version: " v0.2.0 ", want: "v0.2.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalize(tt.version); got != tt.want {
				t.Fatalf("normalize(%q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}

func TestCurrentIsNotEmpty(t *testing.T) {
	if got := Current(); got == "" {
		t.Fatal("Current() returned an empty version")
	}
}
