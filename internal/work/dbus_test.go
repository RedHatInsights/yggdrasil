package work

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestScrubName(t *testing.T) {
	tests := []struct {
		description string
		input       string
		want        string
		wantError   error
	}{
		{
			description: "no hyphens",
			input:       "alpha",
			want:        "alpha",
		},
		{
			description: "single hyphen",
			input:       "alpha-bravo",
			want:        "alpha_bravo",
			wantError:   cmpopts.AnyError,
		},
		{
			description: "multiple hyphens",
			input:       "alpha-bravo-charlie",
			want:        "alpha_bravo_charlie",
			wantError:   cmpopts.AnyError,
		},
		{
			description: "consecutive hyphens",
			input:       "alpha--bravo",
			want:        "alpha__bravo",
			wantError:   cmpopts.AnyError,
		},
		{
			description: "mixed",
			input:       "alpha_bravo-charlie",
			want:        "alpha_bravo_charlie",
			wantError:   cmpopts.AnyError,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			got, err := ScrubName(test.input)

			if !cmp.Equal(got, test.want) {
				t.Errorf("%v", cmp.Diff(got, test.want))
			}

			if !cmp.Equal(err, test.wantError, cmpopts.EquateErrors()) {
				t.Errorf("%#v != %#v", err, test.wantError)
			}
		})
	}
}
