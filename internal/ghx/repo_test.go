package ghx

import (
	"strings"
	"testing"
)

func TestParseRepo(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantOwner string
		wantName  string
		wantErr   bool
	}{
		{name: "valid", in: "owner/repo", wantOwner: "owner", wantName: "repo"},
		{name: "missing slash", in: "noslash", wantErr: true},
		{name: "empty", in: "", wantErr: true},
		{name: "trailing slash", in: "owner/", wantErr: true},
		{name: "extra slash", in: "owner/repo/extra", wantErr: true},
		{name: "empty owner", in: "/repo", wantErr: true},
		{name: "empty name", in: "owner/", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRepo(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseRepo(%q) error = nil, want error", tt.in)
				}
				if !strings.Contains(err.Error(), "invalid repo") {
					t.Fatalf("ParseRepo(%q) error = %q, want invalid repo", tt.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRepo(%q): %v", tt.in, err)
			}
			if got.Owner != tt.wantOwner || got.Name != tt.wantName {
				t.Fatalf("ParseRepo(%q) = %+v, want owner=%q name=%q", tt.in, got, tt.wantOwner, tt.wantName)
			}
			if got.String() != tt.in {
				t.Fatalf("String() = %q, want %q", got.String(), tt.in)
			}
		})
	}
}
