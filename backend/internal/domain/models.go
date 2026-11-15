package domain

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// 全库数值约定：带小数的尺寸/进给/切深以「实际值 × 1000」整数落库。
// 换算只允许走这两个函数。

// MilliToFloat 把落库整数换算成对外十进制。
func MilliToFloat(v int64) float64 { return float64(v) / 1000 }

// FloatToMilli 把对外十进制换算成落库整数（四舍五入到 0.001）。
func FloatToMilli(v float64) int64 {
	if v == 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return int64(math.Round(v * 1000))
}

// Drawing 图纸，模型最上层。
type Drawing struct {
	ID             int64   `db:"id" json:"id"`
	OwnerUserID    int64   `db:"owner_user_id" json:"ownerUserId"`
	OwnerName      string  `db:"owner_name" json:"ownerName"`
	DrawingNo      string  `db:"drawing_no" json:"drawingNo"`
	Name           string  `db:"name" json:"name"` // 产品类型；保留 name 以兼容已有数据和接口。
	Customer       string  `db:"customer" json:"customer"`
	Material       string  `db:"material" json:"material"`
	Price          *string `db:"-" json:"price"` // 元，固定两位小数；nil 为未填写。
	PriceCents     *int64  `db:"price_cents" json:"-"`
	DrawingVersion string  `db:"drawing_version" json:"drawingVersion"`
	Remark         string  `db:"remark" json:"remark"`
	CreatedAt      string  `db:"created_at" json:"createdAt"`
	UpdatedAt      string  `db:"updated_at" json:"updatedAt"`
	OperationCount int     `db:"operation_count" json:"operationCount"`
	ProgramCount   int     `db:"program_count" json:"programCount"`
}

// Operation 工序。装夹方式、分中方式与 X/Y/Z 坐标由同工序下的程序共享。
type Operation struct {
	ID               int64   `db:"id" json:"id"`
	DrawingID        int64   `db:"drawing_id" json:"drawingId"`
	OpNo             int     `db:"op_no" json:"opNo"`
	OpName           string  `db:"op_name" json:"opName"`
	SortOrder        int     `db:"sort_order" json:"sortOrder"`
	MachineID        *int64  `db:"machine_id" json:"machineId"`
	MachineName      string  `db:"machine_name" json:"machineName"`
	Fixture          string  `db:"fixture" json:"fixture"`
	CenteringMethod  string  `db:"centering_method" json:"centeringMethod"`
	XCoordinate      float64 `db:"-" json:"xCoordinate"` // mm
	YCoordinate      float64 `db:"-" json:"yCoordinate"` // mm
	XCoordinateMilli int64   `db:"x_coordinate_milli" json:"-"`
	YCoordinateMilli int64   `db:"y_coordinate_milli" json:"-"`
	ZHeight          float64 `db:"-" json:"zHeight"` // Z 坐标（mm）；沿用旧字段名兼容已有数据与客户端。
	ZHeightMilli     int64   `db:"z_height_milli" json:"-"`
	Remark           string  `db:"remark" json:"remark"`
	CreatedAt        string  `db:"created_at" json:"createdAt"`
	UpdatedAt        string  `db:"updated_at" json:"updatedAt"`
	ProgramCount     int     `db:"program_count" json:"programCount"`
}

// Program 数控程序，挂在工序下。
type Program struct {
	ID               int64  `db:"id" json:"id"`
	Revision         int64  `db:"revision" json:"revision"`
	OperationID      int64  `db:"operation_id" json:"operationId"`
	ProgramNo        string `db:"program_no" json:"programNo"`
	ProgramName      string `db:"program_name" json:"programName"`
	Controller       string `db:"controller" json:"controller"`
	SortOrder        int    `db:"sort_order" json:"sortOrder"`
	CurrentVersionID *int64 `db:"current_version_id" json:"currentVersionId"`
	CurrentVersionNo *int   `db:"current_version_no" json:"currentVersionNo"`
	VersionCount     int    `db:"version_count" json:"versionCount"`
	Remark           string `db:"remark" json:"remark"`
	CreatedAt        string `db:"created_at" json:"createdAt"`
	UpdatedAt        string `db:"updated_at" json:"updatedAt"`
}

// OperationWithPrograms 是图纸详情里的工序节点。
type OperationWithPrograms struct {
	Operation
	Programs []Program `json:"programs"`
}

// DrawingDetail 一次返回图纸 → 工序 → 程序，供图纸页整页渲染。
type DrawingDetail struct {
	Drawing
	Operations []OperationWithPrograms `json:"operations"`
}

// Page 统一分页响应。
type Page[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
	Page  int `json:"page"`
	Size  int `json:"size"`
}

// ---- 请求模型与校验 ----

const (
	maxShortText = 100
	maxRemark    = 2000
)

func checkLen(field, v string, max int) error {
	if len(v) > max {
		return fmt.Errorf("%w: %s 不能超过 %d 字节", ErrInvalid, field, max)
	}
	return nil
}

type DrawingInput struct {
	OwnerUserID    *int64     `json:"ownerUserId,omitempty"`
	Price          PriceInput `json:"price,omitzero"`
	DrawingNo      string     `json:"drawingNo"`
	Name           string     `json:"name"`
	Customer       string     `json:"customer"`
	Material       string     `json:"material"`
	DrawingVersion string     `json:"drawingVersion"`
	Remark         string     `json:"remark"`
}

// Normalize 整理并校验输入。
func (in *DrawingInput) Normalize() error {
	if in.Price.Cents != nil && (*in.Price.Cents < 0 || *in.Price.Cents > MaxPriceCents) {
		return fmt.Errorf("%w: 价格应为 0~999999999.99 元，最多两位小数", ErrInvalid)
	}
	in.DrawingNo = strings.TrimSpace(in.DrawingNo)
	in.Name = strings.TrimSpace(in.Name)
	in.Customer = strings.TrimSpace(in.Customer)
	in.Material = strings.TrimSpace(in.Material)
	in.DrawingVersion = strings.TrimSpace(in.DrawingVersion)
	if in.DrawingNo == "" {
		return fmt.Errorf("%w: 图纸号不能为空", ErrInvalid)
	}
	for _, f := range []struct{ n, v string }{{"图纸号", in.DrawingNo}, {"类型", in.Name}, {"客户", in.Customer}, {"材料", in.Material}, {"图纸版本", in.DrawingVersion}} {
		if err := checkLen(f.n, f.v, maxShortText); err != nil {
			return err
		}
	}
	return checkLen("备注", in.Remark, maxRemark)
}

type OperationInput struct {
	OpNo            int      `json:"opNo"`
	OpName          string   `json:"opName"`
	MachineID       *int64   `json:"machineId"`
	Fixture         string   `json:"fixture"`
	CenteringMethod *string  `json:"centeringMethod"` // 旧客户端未传时，编辑保留已有值。
	XCoordinate     *float64 `json:"xCoordinate"`     // 旧客户端未传时，编辑保留已有值。
	YCoordinate     *float64 `json:"yCoordinate"`
	ZHeight         float64  `json:"zHeight"`
	Remark          string   `json:"remark"`
}

func (in *OperationInput) Normalize() error {
	in.OpName = strings.TrimSpace(in.OpName)
	in.Fixture = strings.TrimSpace(in.Fixture)
	if in.CenteringMethod != nil {
		value := strings.TrimSpace(*in.CenteringMethod)
		if utf8.RuneCountInString(value) > 200 {
			return fmt.Errorf("%w: 分中方式不能超过 200 字", ErrInvalid)
		}
		in.CenteringMethod = &value
	}
	if in.OpNo <= 0 || in.OpNo > 9999 {
		return fmt.Errorf("%w: 工序号应为 1~9999", ErrInvalid)
	}
	if in.MachineID != nil && *in.MachineID <= 0 {
		in.MachineID = nil
	}
	for _, axis := range []struct {
		name  string
		value *float64
	}{{"X", in.XCoordinate}, {"Y", in.YCoordinate}, {"Z", &in.ZHeight}} {
		if axis.value != nil && (math.IsNaN(*axis.value) || math.IsInf(*axis.value, 0) || math.Abs(*axis.value) > 100000) {
			return fmt.Errorf("%w: %s 坐标应为 -100000~100000 mm 的数字", ErrInvalid, axis.name)
		}
	}
	if err := checkLen("工序名称", in.OpName, maxShortText); err != nil {
		return err
	}
	if err := checkLen("装夹方式", in.Fixture, maxShortText*2); err != nil {
		return err
	}
	return checkLen("备注", in.Remark, maxRemark)
}

type ProgramInput struct {
	ExpectedRevision *int64 `json:"expectedRevision,omitempty"`
	ProgramNo        string `json:"programNo"`
	ProgramName      string `json:"programName"`
	Controller       string `json:"controller"`
	Remark           string `json:"remark"`
}

func (in *ProgramInput) Normalize() error {
	in.ProgramNo = strings.TrimSpace(in.ProgramNo)
	in.ProgramName = strings.TrimSpace(in.ProgramName)
	in.Controller = strings.TrimSpace(in.Controller)
	if in.ProgramNo == "" {
		return fmt.Errorf("%w: 程序号不能为空", ErrInvalid)
	}
	for _, f := range []struct{ n, v string }{{"程序号", in.ProgramNo}, {"程序名称", in.ProgramName}, {"数控系统", in.Controller}} {
		if err := checkLen(f.n, f.v, maxShortText); err != nil {
			return err
		}
	}
	return checkLen("备注", in.Remark, maxRemark)
}

// ReorderInput 是拖动排序后提交的完整 ID 顺序。
type ReorderInput struct {
	IDs []int64 `json:"ids"`
}
