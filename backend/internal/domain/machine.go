package domain

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type MachineOption struct {
	ID      int64  `db:"id" json:"id"`
	Code    string `db:"code" json:"code"`
	Name    string `db:"name" json:"name"`
	Enabled bool   `db:"enabled" json:"enabled"`
}

type MachineInput struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Controller  string `json:"controller"`
	Remark      string `json:"remark"`
	OwnerUserID *int64 `json:"ownerUserId"`
}

type Machine struct {
	MachineOption
	Controller     string `db:"controller" json:"controller"`
	Remark         string `db:"remark" json:"remark"`
	OwnerUserID    int64  `db:"owner_user_id" json:"ownerUserId"`
	OwnerName      string `db:"owner_name" json:"ownerName"`
	DrawingCount   int    `db:"drawing_count" json:"drawingCount"`
	OperationCount int    `db:"operation_count" json:"operationCount"`
	ProgramCount   int    `db:"program_count" json:"programCount"`
	CreatedAt      string `db:"created_at" json:"createdAt"`
	UpdatedAt      string `db:"updated_at" json:"updatedAt"`
}

type MachineFilter struct {
	Keyword    string
	Enabled    *bool
	Page, Size int
}

func (in *MachineInput) Normalize() error {
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	in.Controller = strings.TrimSpace(in.Controller)
	in.Remark = strings.TrimSpace(in.Remark)
	if in.Code == "" || utf8.RuneCountInString(in.Code) > 64 {
		return fmt.Errorf("%w: 机台编号应为 1~64 个字符", ErrInvalid)
	}
	if utf8.RuneCountInString(in.Name) > 100 {
		return fmt.Errorf("%w: 机台名称最多 100 个字符", ErrInvalid)
	}
	if utf8.RuneCountInString(in.Controller) > 100 || utf8.RuneCountInString(in.Remark) > 2000 {
		return fmt.Errorf("%w: 控制系统最多 100 个字符，备注最多 2000 个字符", ErrInvalid)
	}
	return nil
}
