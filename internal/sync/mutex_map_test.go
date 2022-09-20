package sync

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestSet(t *testing.T) {
	tests := []struct {
		description string
		input       struct {
			key string
			val string
		}
		want *RWMutexMap[string]
	}{
		{
			input: struct {
				key string
				val string
			}{
				key: "key",
				val: "val",
			},
			want: &RWMutexMap[string]{
				mp: map[string]string{
					"key": "val",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			got := &RWMutexMap[string]{}
			got.Set(test.input.key, test.input.val)

			if !cmp.Equal(got, test.want, cmp.AllowUnexported(RWMutexMap[string]{}), cmpopts.IgnoreFields(RWMutexMap[string]{}, "mu")) {
				t.Errorf("%#v != %#v", got, test.want)
			}
		})
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		description string
		input       struct {
			m *RWMutexMap[string]
			k string
		}
		want string
	}{
		{
			input: struct {
				m *RWMutexMap[string]
				k string
			}{
				m: &RWMutexMap[string]{
					mp: map[string]string{
						"key": "val",
					},
				},
				k: "key",
			},
			want: "val",
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			got, _ := test.input.m.Get(test.input.k)

			if !cmp.Equal(got, test.want) {
				t.Errorf("%v != %v", got, test.want)
			}
		})
	}
}

func TestDel(t *testing.T) {
	tests := []struct {
		description string
		input       struct {
			m *RWMutexMap[string]
			k string
		}
		want *RWMutexMap[string]
	}{
		{
			input: struct {
				m *RWMutexMap[string]
				k string
			}{
				m: &RWMutexMap[string]{
					mp: map[string]string{
						"key1": "val1",
						"key2": "val2",
					},
				},
				k: "key1",
			},
			want: &RWMutexMap[string]{
				mp: map[string]string{
					"key2": "val2",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			got := &RWMutexMap[string]{
				mp: test.input.m.mp,
			}

			got.Del(test.input.k)

			if !cmp.Equal(got, test.want, cmp.AllowUnexported(RWMutexMap[string]{}), cmpopts.IgnoreFields(RWMutexMap[string]{}, "mu")) {
				t.Errorf("%v", cmp.Diff(got, test.want))
			}
		})
	}
}
