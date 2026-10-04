package tray

import (
	"encoding/binary"
	"testing"
)

func TestExitActionRunsOnce(t *testing.T) {
	calls := 0
	actions := guardedActions{actions: Actions{Exit: func() { calls++ }}}
	actions.exit()
	actions.exit()
	if calls != 1 {
		t.Fatalf("退出回调执行了 %d 次，期望 1 次", calls)
	}
}

func TestICOFrameSelectsPreferredSize(t *testing.T) {
	data := make([]byte, 6+2*16+5)
	binary.LittleEndian.PutUint16(data[2:4], 1)
	binary.LittleEndian.PutUint16(data[4:6], 2)
	data[6] = 16
	binary.LittleEndian.PutUint32(data[14:18], 2)
	binary.LittleEndian.PutUint32(data[18:22], uint32(6+2*16))
	data[22] = 32
	binary.LittleEndian.PutUint32(data[30:34], 3)
	binary.LittleEndian.PutUint32(data[34:38], uint32(6+2*16+2))
	copy(data[38:], []byte{1, 2, 3, 4, 5})
	frame, err := icoFrame(data, 32)
	if err != nil {
		t.Fatal(err)
	}
	if len(frame) != 3 || frame[0] != 3 || frame[2] != 5 {
		t.Fatalf("选择了错误的图标帧：%v", frame)
	}
}
