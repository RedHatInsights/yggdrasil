package work

import (
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/redhatinsights/yggdrasil/ipc"
)

func TestWorkerEventFromSignal(t *testing.T) {
	tests := []struct {
		description string
		input       *dbus.Signal
		want        *ipc.WorkerEvent
		wantError   error
	}{
		{
			input: &dbus.Signal{
				Name: "com.redhat.Yggdrasil1.Worker1.Event",
				Body: []interface{}{uint32(1), "6925055f-167a-45cc-9869-1789ee37883f"},
			},
			want: &ipc.WorkerEvent{
				Name:      ipc.WorkerEventNameBegin,
				MessageID: "6925055f-167a-45cc-9869-1789ee37883f",
			},
		},
		{
			input: &dbus.Signal{
				Name: "com.redhat.Yggdrasil1.Worker1.Event",
				Body: []interface{}{uint32(2), "6925055f-167a-45cc-9869-1789ee37883f"},
			},
			want: &ipc.WorkerEvent{
				Name:      ipc.WorkerEventNameEnd,
				MessageID: "6925055f-167a-45cc-9869-1789ee37883f",
			},
		},
		{
			input: &dbus.Signal{
				Name: "com.redhat.Yggdrasil1.Worker1.Event",
				Body: []interface{}{uint32(3), "6925055f-167a-45cc-9869-1789ee37883f"},
			},
			want: &ipc.WorkerEvent{
				Name:      ipc.WorkerEventNameWorking,
				MessageID: "6925055f-167a-45cc-9869-1789ee37883f",
			},
		},
		{
			input: &dbus.Signal{
				Name: "com.redhat.Yggdrasil1.Worker1.Event",
				Body: []interface{}{
					uint32(3),
					"6925055f-167a-45cc-9869-1789ee37883f",
					"working message",
				},
			},
			want: &ipc.WorkerEvent{
				Name:      ipc.WorkerEventNameWorking,
				MessageID: "6925055f-167a-45cc-9869-1789ee37883f",
				Message:   "working message",
			},
		},
		{
			input: &dbus.Signal{
				Name: "com.redhat.Yggdrasil1.Worker1.Event",
				Body: []interface{}{"3", "6925055f-167a-45cc-9869-1789ee37883f"},
			},
			want:      nil,
			wantError: newUint32TypeConversionError("3"),
		},
		{
			input: &dbus.Signal{
				Name: "com.redhat.Yggdrasil1.Worker1.Event",
				Body: []interface{}{uint32(1), 3},
			},
			want:      nil,
			wantError: newStringTypeConversionError(3),
		},
		{
			input: &dbus.Signal{
				Name: "com.redhat.Yggdrasil1.Worker1.Event",
				Body: []interface{}{uint32(3), "6925055f-167a-45cc-9869-1789ee37883f", 3},
			},
			want:      nil,
			wantError: newStringTypeConversionError(3),
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			got, err := workerEventFromSignal(test.input)

			if test.wantError != nil {
				if !cmp.Equal(err, test.wantError, cmpopts.EquateErrors()) {
					t.Errorf("%#v != %#v", err, test.wantError)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if !cmp.Equal(got, test.want) {
					t.Errorf("%v", cmp.Diff(got, test.want))
				}
			}
		})
	}
}
