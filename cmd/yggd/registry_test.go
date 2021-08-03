package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestSet(t *testing.T) {
	tests := []struct {
		description string
		input       []struct {
			handler string
			worker  *worker
		}
		want      *registry
		wantError error
	}{
		{
			description: "two handler values",
			input: []struct {
				handler string
				worker  *worker
			}{
				{
					handler: "test1",
					worker: &worker{
						handler: "test1",
					},
				},
				{
					handler: "test2",
					worker: &worker{
						handler: "test2",
					},
				},
			},
			want: &registry{
				mp: map[string]*worker{
					"test1": {
						handler: "test1",
					},
					"test2": {
						handler: "test2",
					},
				},
			},
		},
		{
			description: "duplicate handler value",
			input: []struct {
				handler string
				worker  *worker
			}{
				{
					handler: "test",
					worker: &worker{
						handler: "test",
					},
				},
				{
					handler: "test",
					worker: &worker{
						handler: "test",
					},
				},
			},
			wantError: errWorkerRegistered,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			got := &registry{}

			for _, input := range test.input {
				if err := got.set(input.handler, input.worker); err != nil {
					if err != nil {
						if test.wantError != nil {
							if !cmp.Equal(err, test.wantError) {
								t.Fatalf("%#v != %#v", err, test.wantError)
							}
						} else {
							t.Fatalf("unexpected error: %v", err)
						}
					}
				}
			}

			if test.wantError == nil {
				if !cmp.Equal(got, test.want, cmp.AllowUnexported(registry{}, worker{}), cmpopts.IgnoreFields(registry{}, "mu")) {
					t.Fatalf("%#v != %#v", got, test.want)
				}
			}
		})
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		description string
		input       struct {
			r       *registry
			handler string
		}
		want *worker
	}{
		{
			description: "present",
			input: struct {
				r       *registry
				handler string
			}{
				r: &registry{
					mp: map[string]*worker{
						"test": {
							handler: "test",
						},
					},
				},
				handler: "test",
			},
			want: &worker{
				handler: "test",
			},
		},
		{
			description: "absent",
			input: struct {
				r       *registry
				handler string
			}{
				r: &registry{
					mp: map[string]*worker{
						"test": {
							handler: "test",
						},
					},
				},
				handler: "test2",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			got := test.input.r.get(test.input.handler)

			if !cmp.Equal(got, test.want, cmp.AllowUnexported(registry{}, worker{}), cmpopts.IgnoreFields(registry{}, "mu")) {
				t.Errorf("%#v != %#v", got, test.want)
			}
		})
	}
}

func TestDel(t *testing.T) {
	tests := []struct {
		description string
		input       struct {
			r *registry
			h string
		}
		want *registry
	}{
		{
			input: struct {
				r *registry
				h string
			}{
				r: &registry{
					mp: map[string]*worker{
						"test": {
							handler: "test",
						},
					},
				},
				h: "test",
			},
			want: &registry{
				mp: map[string]*worker{},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			got := &registry{
				mp: test.input.r.mp,
			}
			got.del(test.input.h)

			if !cmp.Equal(got, test.want, cmp.AllowUnexported(registry{}, worker{}), cmpopts.IgnoreFields(registry{}, "mu")) {
				t.Errorf("%#v != %#v", got, test.want)
			}
		})
	}
}

func TestAll(t *testing.T) {
	tests := []struct {
		description string
		input       *registry
		want        map[string]*worker
	}{
		{
			input: &registry{
				mp: map[string]*worker{
					"test1": {
						handler: "test1",
					},
					"test2": {
						handler: "test2",
					},
				},
			},
			want: map[string]*worker{
				"test1": {
					handler: "test1",
				},
				"test2": {
					handler: "test2",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			got := test.input.all()

			if !cmp.Equal(got, test.want, cmp.AllowUnexported(registry{}, worker{}), cmpopts.IgnoreFields(registry{}, "mu")) {
				t.Errorf("%#v != %#v", got, test.want)
			}
		})
	}
}
