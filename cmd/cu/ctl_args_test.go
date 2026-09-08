package main

import "testing"

func TestParseCtlArgs(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantCmd   string
		wantQuiet bool
		wantErr   bool
	}{
		{"resume then quiet", []string{"resume", "--quiet"}, "resume", true, false},
		{"quiet then resume", []string{"--quiet", "resume"}, "resume", true, false},
		{"bogus command with quiet", []string{"bogus", "--quiet"}, "bogus", true, true},
		{"no args", nil, "", false, true},
		{"status no quiet", []string{"status"}, "status", false, false},
		{"quiet only, no command", []string{"--quiet"}, "", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, quiet, err := parseCtlArgs(tc.args)
			if cmd != tc.wantCmd {
				t.Errorf("cmd = %q, want %q", cmd, tc.wantCmd)
			}
			if quiet != tc.wantQuiet {
				t.Errorf("quiet = %v, want %v", quiet, tc.wantQuiet)
			}
			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
