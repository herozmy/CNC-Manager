package domain

import (
	"encoding/hex"
	"fmt"
	"math"
	"strings"
)

// ProgramTool 程序用刀 + 刀补参数。整个软件最核心、现场最容易出错的一张表。
//
// 数值单位：ToolDia / CornerRadius / CutDepth 单位 mm；
// SpindleSpeed 按 SpeedMode 解释；Feed 按 FeedMode 解释。
// 落库时统一乘 1000 存整数（见 store/migrations）。
type ProgramTool struct {
	ID        int64  `db:"id" json:"id"`
	ProgramID int64  `db:"program_id" json:"programId"`
	VersionID int64  `db:"version_id" json:"versionId"`
	Seq       int    `db:"seq" json:"seq"`
	ToolID    *int64 `db:"tool_id" json:"toolId"`

	ToolNo           string            `db:"tool_no" json:"toolNo"`     // T 号，如 T01
	OffsetNo         string            `db:"offset_no" json:"offsetNo"` // 刀补号，如 D01 / H01
	ToolName         string            `db:"tool_name" json:"toolName"`
	ToolDia          float64           `db:"-" json:"toolDia"`      // mm
	CornerRadius     float64           `db:"-" json:"cornerRadius"` // mm
	CompAmount       float64           `db:"-" json:"compAmount"`   // 刀具补偿量，mm
	SpindleSpeed     int               `db:"spindle_speed" json:"spindleSpeed"`
	SpeedMode        int               `db:"speed_mode" json:"speedMode"` // 0=G97 r/min, 1=G96 m/min
	Feed             float64           `db:"-" json:"feed"`
	FeedMode         int               `db:"feed_mode" json:"feedMode"` // 0=G94 mm/min, 1=G95 mm/r
	CutDepth         float64           `db:"-" json:"cutDepth"`         // mm
	Coolant          int               `db:"coolant" json:"coolant"`    // 0=无 1=冷却液 2=吹气 3=喷雾
	MachiningContent string            `db:"machining_content" json:"machiningContent"`
	Remark           string            `db:"remark" json:"remark"`
	CustomParams     []ToolCustomParam `db:"-" json:"customParams"`
	CustomParamsJSON string            `db:"custom_params_json" json:"-"`

	// 落库字段（整数，值为实际值 × 1000），不直接暴露给前端
	ToolDiaMilli      int64 `db:"tool_dia_milli" json:"-"`
	CornerRadiusMilli int64 `db:"corner_radius_milli" json:"-"`
	CompAmountMilli   int64 `db:"comp_amount_milli" json:"-"`
	FeedMilli         int64 `db:"feed_milli" json:"-"`
	CutDepthMilli     int64 `db:"cut_depth_milli" json:"-"`
}

// ToolCustomParam 是用户为某把刀补充的自定义展示项，例如「品牌 / 山特维克」。
type ToolCustomParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ProgramToolInput 是前端提交的刀具行，数值为人类单位（mm / r/min）。
type ProgramToolInput struct {
	Seq              int               `json:"seq"`
	ToolID           *int64            `json:"toolId"`
	ToolNo           string            `json:"toolNo"`
	OffsetNo         string            `json:"offsetNo"`
	ToolName         string            `json:"toolName"`
	ToolDia          float64           `json:"toolDia"`
	CornerRadius     float64           `json:"cornerRadius"`
	CompAmount       float64           `json:"compAmount"` // 刀具补偿量，mm
	SpindleSpeed     int               `json:"spindleSpeed"`
	SpeedMode        int               `json:"speedMode"`
	Feed             float64           `json:"feed"`
	FeedMode         int               `json:"feedMode"`
	CutDepth         float64           `json:"cutDepth"`
	Coolant          int               `json:"coolant"`
	MachiningContent string            `json:"machiningContent"`
	Remark           string            `json:"remark"`
	CustomParams     []ToolCustomParam `json:"customParams"`
}

const MaxToolRows = 500

type ToolTable struct {
	Items    []ProgramTool `json:"items"`
	Revision string        `json:"revision"`
}

type ToolTableInput struct {
	Items            []ProgramToolInput `json:"items"`
	ExpectedRevision string             `json:"expectedRevision"`
}

func (in *ToolTableInput) Normalize() error {
	revision, err := hex.DecodeString(in.ExpectedRevision)
	if err != nil || len(revision) != 32 {
		return fmt.Errorf("%w: 请先加载刀具表再保存", ErrInvalid)
	}
	if in.Items == nil {
		return fmt.Errorf("%w: items 必须为数组，清空请传 []", ErrInvalid)
	}
	if len(in.Items) > MaxToolRows {
		return fmt.Errorf("%w: 刀具表最多 %d 行", ErrInvalid, MaxToolRows)
	}
	for i := range in.Items {
		row := &in.Items[i]
		row.Seq = i + 1
		row.ToolNo = strings.TrimSpace(row.ToolNo)
		row.OffsetNo = strings.TrimSpace(row.OffsetNo)
		if err := row.validate(); err != nil {
			return fmt.Errorf("第 %d 行：%w", i+1, err)
		}
		if row.CustomParams == nil {
			row.CustomParams = []ToolCustomParam{}
		}
	}
	return nil
}

func (in *ProgramToolInput) validate() error {
	if in.ToolNo == "" {
		return fmt.Errorf("%w: 刀具号不能为空", ErrInvalid)
	}
	if in.ToolID != nil && *in.ToolID <= 0 {
		return fmt.Errorf("%w: 刀具关联无效", ErrInvalid)
	}
	for _, f := range []struct{ n, v string }{{"刀具号", in.ToolNo}, {"刀补号", in.OffsetNo}, {"刀具名称", in.ToolName}} {
		if err := checkLen(f.n, f.v, 100); err != nil {
			return err
		}
	}
	for _, f := range []struct{ n, v string }{{"加工内容", in.MachiningContent}, {"备注", in.Remark}} {
		if err := checkLen(f.n, f.v, 2000); err != nil {
			return err
		}
	}
	for _, f := range []struct {
		n           string
		v, min, max float64
	}{
		{"直径", in.ToolDia, 0, 100000}, {"刀尖圆角", in.CornerRadius, 0, 100000},
		{"补偿量", in.CompAmount, -100000, 100000}, {"进给", in.Feed, 0, 1000000}, {"切深", in.CutDepth, 0, 100000},
	} {
		if math.IsNaN(f.v) || math.IsInf(f.v, 0) || f.v < f.min || f.v > f.max {
			return fmt.Errorf("%w: %s 应为 %g~%g 的数字", ErrInvalid, f.n, f.min, f.max)
		}
	}
	if in.SpindleSpeed < 0 || in.SpindleSpeed > 1000000 {
		return fmt.Errorf("%w: 主轴速度应为 0~1000000 的整数", ErrInvalid)
	}
	if in.SpeedMode < 0 || in.SpeedMode > 1 || in.FeedMode < 0 || in.FeedMode > 1 || in.Coolant < 0 || in.Coolant > 3 {
		return fmt.Errorf("%w: 速度、进给或冷却模式无效", ErrInvalid)
	}
	if len(in.CustomParams) > 20 {
		return fmt.Errorf("%w: 自定义参数最多 20 项", ErrInvalid)
	}
	seen := map[string]bool{}
	for _, p := range in.CustomParams {
		name := strings.ToLower(strings.TrimSpace(p.Name))
		if name == "" || seen[name] {
			return fmt.Errorf("%w: 自定义参数名称不能为空或重复", ErrInvalid)
		}
		seen[name] = true
		if err := checkLen("参数名称", p.Name, 100); err != nil {
			return err
		}
		if err := checkLen("参数值", p.Value, 600); err != nil {
			return err
		}
	}
	return nil
}

func (t *ProgramTool) ApplyMilli() {
	t.ToolDia = MilliToFloat(t.ToolDiaMilli)
	t.CornerRadius = MilliToFloat(t.CornerRadiusMilli)
	t.CompAmount = MilliToFloat(t.CompAmountMilli)
	t.Feed = MilliToFloat(t.FeedMilli)
	t.CutDepth = MilliToFloat(t.CutDepthMilli)
}
