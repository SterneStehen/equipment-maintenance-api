package main

import "testing"

func TestParseDownSteps(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		want      int
		wantError bool
	}{
		{name: "one step", arguments: []string{"down", "1"}, want: 1},
		{name: "several steps", arguments: []string{"down", "3"}, want: 3},
		{name: "missing steps", arguments: []string{"down"}, wantError: true},
		{name: "zero steps", arguments: []string{"down", "0"}, wantError: true},
		{name: "invalid steps", arguments: []string{"down", "all"}, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := downSteps(test.arguments)
			if (err != nil) != test.wantError {
				t.Fatalf("downSteps() error = %v, wantError = %t", err, test.wantError)
			}
			if got != test.want {
				t.Errorf("downSteps() = %d, want %d", got, test.want)
			}
		})
	}
}
