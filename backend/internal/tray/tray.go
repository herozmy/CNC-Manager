package tray

import (
	"encoding/binary"
	"fmt"
	"sync"
)

type Actions struct {
	OpenApp  func()
	OpenLogs func()
	Exit     func()
}

func icoFrame(data []byte, preferredSize int) ([]byte, error) {
	if len(data) < 6 || binary.LittleEndian.Uint16(data[0:2]) != 0 || binary.LittleEndian.Uint16(data[2:4]) != 1 {
		return nil, fmt.Errorf("invalid ico header")
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	if count == 0 || len(data) < 6+count*16 {
		return nil, fmt.Errorf("invalid ico directory")
	}
	bestIndex := -1
	bestDistance := int(^uint(0) >> 1)
	for index := 0; index < count; index++ {
		entry := data[6+index*16 : 6+(index+1)*16]
		size := int(entry[0])
		if size == 0 {
			size = 256
		}
		distance := size - preferredSize
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance {
			bestIndex = index
			bestDistance = distance
		}
	}
	entry := data[6+bestIndex*16 : 6+(bestIndex+1)*16]
	length := int(binary.LittleEndian.Uint32(entry[8:12]))
	offset := int(binary.LittleEndian.Uint32(entry[12:16]))
	if length <= 0 || offset < 0 || offset > len(data)-length {
		return nil, fmt.Errorf("invalid ico frame")
	}
	return data[offset : offset+length], nil
}

type guardedActions struct {
	actions  Actions
	exitOnce sync.Once
}

func (a *guardedActions) openApp() {
	if a.actions.OpenApp != nil {
		a.actions.OpenApp()
	}
}

func (a *guardedActions) openLogs() {
	if a.actions.OpenLogs != nil {
		a.actions.OpenLogs()
	}
}

func (a *guardedActions) exit() {
	a.exitOnce.Do(func() {
		if a.actions.Exit != nil {
			a.actions.Exit()
		}
	})
}

func Start(actions Actions) (func(), error) {
	return startPlatform(&guardedActions{actions: actions})
}
