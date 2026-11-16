package domain

import (
	"encoding/json"
	"fmt"
)

// MachineReference distinguishes an omitted field (preserve on update) from
// explicit null (unlink). IDs must be positive integer JSON numbers.
type MachineReference struct {
	Set bool
	ID  *int64
}

func (m MachineReference) IsZero() bool                 { return !m.Set }
func (m MachineReference) MarshalJSON() ([]byte, error) { return json.Marshal(m.ID) }
func (m *MachineReference) UnmarshalJSON(raw []byte) error {
	*m = MachineReference{Set: true}
	if err := json.Unmarshal(raw, &m.ID); err != nil {
		return fmt.Errorf("%w: 机台标识应为正整数或 null", ErrInvalid)
	}
	if m.ID != nil && *m.ID <= 0 {
		return fmt.Errorf("%w: 机台标识应为正整数", ErrInvalid)
	}
	return nil
}
