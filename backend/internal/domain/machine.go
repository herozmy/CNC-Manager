package domain

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type MachineOption struct {
	ID   int64  `db:"id" json:"id"`
	Code string `db:"code" json:"code"`
	Name string `db:"name" json:"name"`
}

type MachineInput struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

func (in *MachineInput) Normalize() error {
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	if in.Code == "" || utf8.RuneCountInString(in.Code) > 64 {
		return fmt.Errorf("%w: 机台编号应为 1~64 个字符", ErrInvalid)
	}
	if utf8.RuneCountInString(in.Name) > 100 {
		return fmt.Errorf("%w: 机台名称最多 100 个字符", ErrInvalid)
	}
	return nil
}
