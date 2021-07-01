package tags

import (
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestReadTags(t *testing.T) {
	tests := []struct {
		description string
		input       io.Reader
		want        map[string]string
		wantError   error
	}{
		{
			description: "valid",
			input: strings.NewReader(strings.Join([]string{`production = false`,
				`region = "us-east1"`,
				`priority = 3`,
				`uptime = 99.999`,
				`provisioned = 2006-01-02T15:04:05-07:00`,
				`updated = 2006-01-02`}, "\n")),
			want: map[string]string{
				"production":  "false",
				"region":      "us-east1",
				"priority":    "3",
				"uptime":      "99.999",
				"provisioned": "2006-01-02T15:04:05-07:00",
				"updated":     "2006-01-02",
			},
		},
		{
			description: "invalid - table",
			input:       strings.NewReader(strings.Join([]string{`[test]`, `key = "value"`}, "\n")),
			wantError:   &errorTag{map[string]interface{}{}},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			got, err := ReadTags(test.input)

			if test.wantError != nil {
				if !cmp.Equal(err, test.wantError, cmpopts.EquateErrors()) {
					t.Errorf("%#v != %#v", err, test.wantError)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if !cmp.Equal(got, test.want) {
					t.Errorf("%#v != %#v", got, test.want)
				}
			}
		})
	}
}
