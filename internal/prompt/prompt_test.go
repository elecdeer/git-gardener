package prompt

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfirm(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"YES\n", true},
		{" y \n", true},
		{"n\n", false},
		{"\n", false},
		{"", false},
		{"yes", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var out bytes.Buffer

			got, err := Confirm(strings.NewReader(tt.input), &out, "Proceed?")

			if err != nil || got != tt.want {
				t.Errorf("got (%v, %v), want %v", got, err, tt.want)
			}
			if !strings.HasPrefix(out.String(), "Proceed? [y/N]: ") {
				t.Errorf("prompt = %q", out.String())
			}
		})
	}
}
