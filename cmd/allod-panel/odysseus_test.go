package main

import "testing"

func TestExtractOdysseusTempPassword(t *testing.T) {
	tests := []struct {
		name string
		logs string
		want string
	}{
		{"empty", "", ""},
		{"no password", "INFO: starting uvicorn\nINFO: ready", ""},
		{"temporary password", "INFO: Created admin account\nTemporary password: Xy7-abc_123\nINFO: ready", "Xy7-abc_123"},
		{"case and equals", "TEMPORARY PASSWORD = s3cret!", "s3cret!"},
		{"boxed output", "║ Temporary password: Abc123XYZ ║", "Abc123XYZ"},
		{"ansi codes", "\x1b[33mTemporary password: Zz99\x1b[0m", "Zz99"},
		{"quoted", `Temporary password: "quoted123"`, "quoted123"},
		{"latest wins", "Temporary password: old111\nrestart\nTemporary password: new222", "new222"},
		{"generic fallback", "admin password: fallback42", "fallback42"},
		{"temporary preferred over generic", "db password: dbpass\nTemporary password: adminpass", "adminpass"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractOdysseusTempPassword(tt.logs); got != tt.want {
				t.Errorf("extractOdysseusTempPassword() = %q, want %q", got, tt.want)
			}
		})
	}
}
