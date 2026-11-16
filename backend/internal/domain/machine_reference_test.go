package domain

import (
	"encoding/json"
	"testing"
)

func TestProgramMachineReference(t *testing.T) {
	for _, tc := range []struct {
		raw string
		set bool
		id  int64
		bad bool
	}{{`{}`, false, 0, false}, {`{"machineId":null}`, true, 0, false}, {`{"machineId":5}`, true, 5, false}, {`{"machineId":-1}`, true, 0, true}, {`{"machineId":0}`, true, 0, true}, {`{"machineId":"5"}`, true, 0, true}, {`{"machineId":1.2}`, true, 0, true}} {
		var in ProgramInput
		err := json.Unmarshal([]byte(tc.raw), &in)
		if (err != nil) != tc.bad {
			t.Fatal(tc, err)
		}
		if tc.bad {
			continue
		}
		if in.MachineID.Set != tc.set || (tc.id == 0 && in.MachineID.ID != nil) || (tc.id > 0 && (in.MachineID.ID == nil || *in.MachineID.ID != tc.id)) {
			t.Fatal(tc, in)
		}
	}
}
