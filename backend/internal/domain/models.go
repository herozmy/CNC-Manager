// Package domain 定义领域实体与接口请求模型。
//
// 这一层不依赖数据库、HTTP 框架和任何第三方库，是业务规则的唯一真源。
// 换数据库、换 Web 框架都不会动到这里。
package domain

import (
	"errors"
	"time"
)

// 领域错误。repo 层返回它们，httpapi 层统一映射成 HTTP 状态码。
var (
	ErrNotFound = errors.New("记录不存在")
	ErrConflict = errors.New("记录已存在，或与现有数据冲突")
	ErrInvalid  = errors.New("参数不合法")
)

// Now 返回全库统一的时间戳：UTC + RFC3339。
//
// 用文本存时间有两个好处：字典序即时间序，且任何数据库都能原样搬运。
// 换 PostgreSQL 时只需一次 CAST(... AS timestamptz)。
func Now() string { return time.Now().UTC().Format(time.RFC3339) }

// ---------------------------------------------------------------------------
// 实体
// ---------------------------------------------------------------------------

// Drawing 图纸，整个数据模型的最上层。
type Drawing struct {
	ID             int64  `db:"id" json:"id"`
	DrawingNo      string `db:"drawing_no" json:"drawingNo"`
	Name           string `db:"name" json:"name"`
	Customer       string `db:"customer" json:"customer"`
	Material       string `db:"material" json:"material"`
	DrawingVersion string `db:"drawing_version" json:"drawingVersion"`
	Remark         string `db:"remark" json:"remark"`
	CreatedAt      string `db:"created_at" json:"createdAt"`
	UpdatedAt      string `db:"updated_at" json:"updatedAt"`

	OperationCount int `db:"operation_count" json:"operationCount"`
}

// Operation 工序（一序 / 二序 …），界面上显示为 10#、20#、30#。
//
// OpNo 用 10 / 20 / 30 这种编号，中间留空位，
// 将来要在两道工序之间插一道就不用把后面的全部重排。
//
// 装夹方式、Z 轴垫高、备注都属于「这道工序怎么装夹」的物理事实，
// 所以挂在工序上而不是程序上——同一道工序下的多个程序共享这些信息。
type Operation struct {
	ID          int64   `db:"id" json:"id"`
	DrawingID   int64   `db:"drawing_id" json:"drawingId"`
	OpNo        int     `db:"op_no" json:"opNo"`
	OpName      string  `db:"op_name" json:"opName"`
	MachineID   *int64  `db:"machine_id" json:"machineId"`
	MachineName string  `db:"machine_name" json:"machineName"`
	Fixture     string  `db:"fixture" json:"fixture"`
	ZHeight     float64 `db:"-" json:"zHeight"` // Z 轴高度垫高，mm
	Remark      string  `db:"remark" json:"remark"`
	CreatedAt   string  `db:"created_at" json:"createdAt"`
	UpdatedAt   string  `db:"updated_at" json:"updatedAt"`

	ProgramCount int `db:"program_count" json:"programCount"`

	// 落库字段（整数，实际值 × 1000），不直接暴露给前端
	ZHeightMilli int64 `db:"z_height_milli" json:"-"`
}

// Program 数控程序，挂在某道工序下面。
type Program struct {
	ID               int64  `db:"id" json:"id"`
	OperationID      int64  `db:"operation_id" json:"operationId"`
	ProgramNo        string `db:"program_no" json:"programNo"`
	ProgramName      string `db:"program_name" json:"programName"`
	Controller       string `db:"controller" json:"controller"`
	CurrentVersionID *int64 `db:"current_version_id" json:"currentVersionId"`
	Remark           string `db:"remark" json:"remark"`
	CreatedAt        string `db:"created_at" json:"createdAt"`
	UpdatedAt        string `db:"updated_at" json:"updatedAt"`

	CurrentVersionNo *int `db:"current_version_no" json:"currentVersionNo"`
	VersionCount     int  `db:"version_count" json:"versionCount"`
}

// NCFile 物理文件记录，按内容（sha256）寻址，天然去重。
//
// 注意：数据库里存的 RelPath 是相对 NC 库根目录的路径，不是绝对路径。
// 这样整个数据目录可以被整体搬走 / 挂载到容器里，路径永远不会失效。
type NCFile struct {
	ID           int64  `db:"id" json:"id"`
	SHA256       string `db:"sha256" json:"sha256"`
	SizeBytes    int64  `db:"size_bytes" json:"sizeBytes"`
	OriginalName string `db:"original_name" json:"originalName"`
	RelPath      string `db:"rel_path" json:"relPath"`
	CreatedAt    string `db:"created_at" json:"createdAt"`
}

// Version 程序版本。每次上传 NC 文件产生一个新版本，可回滚、可对比。
type Version struct {
	ID         int64  `db:"id" json:"id"`
	ProgramID  int64  `db:"program_id" json:"programId"`
	VersionNo  int    `db:"version_no" json:"versionNo"`
	FileID     int64  `db:"file_id" json:"fileId"`
	ChangeNote string `db:"change_note" json:"changeNote"`
	CreatedBy  string `db:"created_by" json:"createdBy"`
	CreatedAt  string `db:"created_at" json:"createdAt"`

	FileName string `db:"file_name" json:"fileName"`
	FileSize int64  `db:"file_size" json:"fileSize"`
	SHA256   string `db:"sha256" json:"sha256"`
	IsCurrent bool  `db:"is_current" json:"isCurrent"`
}

// ProgramTool 程序用刀 + 刀补参数。整个软件最核心、现场最容易出错的一张表。
//
// 数值单位：ToolDia / CornerRadius / CutDepth 单位 mm；
// SpindleSpeed 按 SpeedMode 解释；Feed 按 FeedMode 解释。
// 落库时统一乘 1000 存整数（见 store/migrations）。
type ProgramTool struct {
	ID        int64  `db:"id" json:"id"`
	ProgramID int64  `db:"program_id" json:"programId"`
	Seq       int    `db:"seq" json:"seq"`
	ToolID    *int64 `db:"tool_id" json:"toolId"`

	ToolNo     string `db:"tool_no" json:"toolNo"`       // T 号，如 T01
	OffsetNo   string `db:"offset_no" json:"offsetNo"`   // 刀补号，如 D01 / H01
	ToolName   string `db:"tool_name" json:"toolName"`
	ToolDia    float64 `db:"-" json:"toolDia"`           // mm
	CornerRadius float64 `db:"-" json:"cornerRadius"`    // mm
	CompAmount   float64 `db:"-" json:"compAmount"`      // 刀具补偿量，mm
	SpindleSpeed int   `db:"spindle_speed" json:"spindleSpeed"`
	SpeedMode    int   `db:"speed_mode" json:"speedMode"` // 0=G97 r/min, 1=G96 m/min
	Feed         float64 `db:"-" json:"feed"`
	FeedMode     int   `db:"feed_mode" json:"feedMode"`   // 0=G94 mm/min, 1=G95 mm/r
	CutDepth     float64 `db:"-" json:"cutDepth"`         // mm
	Coolant      int   `db:"coolant" json:"coolant"`      // 0=无 1=冷却液 2=吹气 3=喷雾
	MachiningContent string `db:"machining_content" json:"machiningContent"`
	Remark           string `db:"remark" json:"remark"`

	// 落库字段（整数，值为实际值 × 1000），不直接暴露给前端
	ToolDiaMilli      int64 `db:"tool_dia_milli" json:"-"`
	CornerRadiusMilli int64 `db:"corner_radius_milli" json:"-"`
	CompAmountMilli   int64 `db:"comp_amount_milli" json:"-"`
	FeedMilli         int64 `db:"feed_milli" json:"-"`
	CutDepthMilli     int64 `db:"cut_depth_milli" json:"-"`
}

// Tool 刀具字典，供程序用刀表下拉复用，避免各人乱填刀具名称。
type Tool struct {
	ID       int64  `db:"id" json:"id"`
	ToolNo   string `db:"tool_no" json:"toolNo"`
	Name     string `db:"name" json:"name"`
	Spec     string `db:"spec" json:"spec"`
	ToolType string `db:"tool_type" json:"toolType"`
	Remark   string `db:"remark" json:"remark"`
	CreatedAt string `db:"created_at" json:"createdAt"`
	UpdatedAt string `db:"updated_at" json:"updatedAt"`
}

// Machine 机台字典。
type Machine struct {
	ID         int64  `db:"id" json:"id"`
	Code       string `db:"code" json:"code"`
	Name       string `db:"name" json:"name"`
	Controller string `db:"controller" json:"controller"`
	Remark     string `db:"remark" json:"remark"`
	CreatedAt  string `db:"created_at" json:"createdAt"`
	UpdatedAt  string `db:"updated_at" json:"updatedAt"`
}

// AuditLog 操作日志。一期没有登录，Actor 固定为 local；
// 后期加登录后直接用同一张表记录"谁改了哪一版"。
type AuditLog struct {
	ID         int64  `db:"id" json:"id"`
	Action     string `db:"action" json:"action"`
	EntityType string `db:"entity_type" json:"entityType"`
	EntityID   int64  `db:"entity_id" json:"entityId"`
	Detail     string `db:"detail" json:"detail"`
	Actor      string `db:"actor" json:"actor"`
	CreatedAt  string `db:"created_at" json:"createdAt"`
}

// ---------------------------------------------------------------------------
// 左树专用视图模型（一次请求返回三级轻量结构，避免前端展开时几十个请求）
// ---------------------------------------------------------------------------

type TreeProgram struct {
	ID               int64  `json:"id"`
	ProgramNo        string `json:"programNo"`
	ProgramName      string `json:"programName"`
	Controller       string `json:"controller"`
	CurrentVersionNo *int   `json:"currentVersionNo"`
	VersionCount     int    `json:"versionCount"`
}

type TreeOperation struct {
	ID           int64         `json:"id"`
	OpNo         int           `json:"opNo"`
	OpName       string        `json:"opName"`
	MachineID    *int64        `json:"machineId"`
	MachineName  string        `json:"machineName"`
	ProgramCount int           `json:"programCount"`
	Programs     []TreeProgram `json:"programs"`
}

type TreeDrawing struct {
	ID             int64           `json:"id"`
	DrawingNo      string          `json:"drawingNo"`
	Name           string          `json:"name"`
	Material       string          `json:"material"`
	Customer       string          `json:"customer"`
	OperationCount int             `json:"operationCount"`
	Operations     []TreeOperation `json:"operations"`
}

// ProgramDetail 程序详情，一次性返回基本信息 + 上下文 + 刀具表 + 版本列表。
type ProgramDetail struct {
	Program
	Drawing   Drawing       `json:"drawing"`
	Operation Operation     `json:"operation"`
	Tools     []ProgramTool `json:"tools"`
	Versions  []Version     `json:"versions"`
}

// ---------------------------------------------------------------------------
// 图纸详情：界面上选中一个图纸，只发这一个请求就把整页要的数据全拿到
//
// 结构完全对应界面的从上到下顺序：
//   图纸 → 工序（装夹方式 / Z轴垫高 / 备注）→ 程序 → 刀具补偿表
// 这样前端不需要逐级请求，也不需要在浏览器里拼接数据。
// ---------------------------------------------------------------------------

// DetailProgram 是图纸详情里的程序节点，直接带上它的刀具补偿表。
type DetailProgram struct {
	ID               int64         `json:"id"`
	ProgramNo        string        `json:"programNo"`
	ProgramName      string        `json:"programName"`
	Controller       string        `json:"controller"`
	CurrentVersionID *int64        `json:"currentVersionId"`
	CurrentVersionNo *int          `json:"currentVersionNo"`
	VersionCount     int           `json:"versionCount"`
	Remark           string        `json:"remark"`
	Tools            []ProgramTool `json:"tools"`
}

// DetailOperation 是图纸详情里的工序节点。
type DetailOperation struct {
	ID          int64           `json:"id"`
	OpNo        int             `json:"opNo"`
	OpName      string          `json:"opName"`
	MachineID   *int64          `json:"machineId"`
	MachineName string          `json:"machineName"`
	Fixture     string          `json:"fixture"`
	ZHeight     float64         `json:"zHeight"`
	Remark      string          `json:"remark"`
	Programs    []DetailProgram `json:"programs"`
}

// DrawingDetail 是一整张图纸的完整内容。
type DrawingDetail struct {
	Drawing    Drawing           `json:"drawing"`
	Operations []DetailOperation `json:"operations"`
}

// ---------------------------------------------------------------------------
// 程序文本的查看与编辑
// ---------------------------------------------------------------------------

// VersionContent 是某个版本的程序文本，供界面上查看和编辑。
//
// Content 已经按 Encoding 解码成 UTF-8，前端直接显示即可；
// 保存时必须把 Encoding 原样带回来，后端才能按同样的编码写回去。
type VersionContent struct {
	VersionID int64  `json:"versionId"`
	ProgramID int64  `json:"programId"`
	VersionNo int    `json:"versionNo"`
	FileName  string `json:"fileName"`
	Encoding  string `json:"encoding"` // 'utf-8' 或 'gbk'
	Content   string `json:"content"`
	LineCount int    `json:"lineCount"`
	SizeBytes int64  `json:"sizeBytes"`
	IsCurrent bool   `json:"isCurrent"`
}

// ContentInput 是保存程序文本的请求体。
type ContentInput struct {
	Content    string `json:"content"`
	Encoding   string `json:"encoding"`
	ChangeNote string `json:"changeNote"`
}

// VersionSaveResult 是保存程序文本的响应。
//
// 嵌入 Version 让原有字段保持在 JSON 同一层（Go 的匿名嵌入会被扁平化），
// 额外带上 warning：告诉用户有哪些字符因为目标编码存不下而被替换了。
// 这类问题肉眼几乎看不出来，必须显式提示。
type VersionSaveResult struct {
	Version
	Warning string `json:"warning,omitempty"`
}

// ---------------------------------------------------------------------------
// 接口请求模型
// ---------------------------------------------------------------------------

type DrawingInput struct {
	DrawingNo      string `json:"drawingNo"`
	Name           string `json:"name"`
	Customer       string `json:"customer"`
	Material       string `json:"material"`
	DrawingVersion string `json:"drawingVersion"`
	Remark         string `json:"remark"`
}

type OperationInput struct {
	OpNo      int     `json:"opNo"`
	OpName    string  `json:"opName"`
	MachineID *int64  `json:"machineId"`
	Fixture   string  `json:"fixture"`
	ZHeight   float64 `json:"zHeight"`
	Remark    string  `json:"remark"`
}

type ProgramInput struct {
	ProgramNo   string `json:"programNo"`
	ProgramName string `json:"programName"`
	Controller  string `json:"controller"`
	Remark      string `json:"remark"`
}

// ProgramToolInput 是前端提交的刀具行，数值为人类单位（mm / r/min）。
type ProgramToolInput struct {
	Seq              int     `json:"seq"`
	ToolID           *int64  `json:"toolId"`
	ToolNo           string  `json:"toolNo"`
	OffsetNo         string  `json:"offsetNo"`
	ToolName         string  `json:"toolName"`
	ToolDia          float64 `json:"toolDia"`
	CornerRadius     float64 `json:"cornerRadius"`
	CompAmount       float64 `json:"compAmount"` // 刀具补偿量，mm
	SpindleSpeed     int     `json:"spindleSpeed"`
	SpeedMode        int     `json:"speedMode"`
	Feed             float64 `json:"feed"`
	FeedMode         int     `json:"feedMode"`
	CutDepth         float64 `json:"cutDepth"`
	Coolant          int     `json:"coolant"`
	MachiningContent string  `json:"machiningContent"`
	Remark           string  `json:"remark"`
}

type ToolInput struct {
	ToolNo   string `json:"toolNo"`
	Name     string `json:"name"`
	Spec     string `json:"spec"`
	ToolType string `json:"toolType"`
	Remark   string `json:"remark"`
}

type MachineInput struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Controller string `json:"controller"`
	Remark     string `json:"remark"`
}

// Page 统一分页响应。
type Page[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
	Page  int `json:"page"`
	Size  int `json:"size"`
}

// DiffLine 两个 NC 版本之间的一行差异。
type DiffLine struct {
	Type      string `json:"type"` // same / add / del / change
	LeftNo    *int   `json:"leftNo"`
	RightNo   *int   `json:"rightNo"`
	LeftText  string `json:"leftText"`
	RightText string `json:"rightText"`
}

// DiffResult 版本对比结果。
type DiffResult struct {
	LeftVersionNo  int        `json:"leftVersionNo"`
	RightVersionNo int        `json:"rightVersionNo"`
	Identical      bool       `json:"identical"`
	Lines          []DiffLine `json:"lines"`
}
