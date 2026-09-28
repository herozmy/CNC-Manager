package domain

import "math"

// 全库数值约定：所有带小数的尺寸 / 进给 / 切深，都以「实际值 × 1000」的整数落库。
//
// 原因：SQLite 没有精确小数类型（NUMERIC 会退化成浮点），
// 直接用浮点存刀补，累计计算后会出现 12.499999 这种脏数据，现场是要出事的。
// 整数则精确、可比较、可排序，且换成 PostgreSQL 的 numeric 时也能无误差搬运。
//
// 换算只允许走下面这两个函数，禁止在别处零散地写 *1000 / /1000。

// MilliToFloat 把落库整数换算成对外的十进制数值。
func MilliToFloat(v int64) float64 { return float64(v) / 1000 }

// FloatToMilli 把对外十进制数值换算成落库整数（四舍五入到 0.001）。
func FloatToMilli(v float64) int64 {
	if v == 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return int64(math.Round(v * 1000))
}

// ApplyMilli 把落库整数填进对外的十进制字段。查询后必须调用。
func (t *ProgramTool) ApplyMilli() {
	t.ToolDia = MilliToFloat(t.ToolDiaMilli)
	t.CornerRadius = MilliToFloat(t.CornerRadiusMilli)
	t.CompAmount = MilliToFloat(t.CompAmountMilli)
	t.Feed = MilliToFloat(t.FeedMilli)
	t.CutDepth = MilliToFloat(t.CutDepthMilli)
}

// FillMilli 把对外十进制字段换算成落库整数。写库前必须调用。
func (t *ProgramTool) FillMilli() {
	t.ToolDiaMilli = FloatToMilli(t.ToolDia)
	t.CornerRadiusMilli = FloatToMilli(t.CornerRadius)
	t.CompAmountMilli = FloatToMilli(t.CompAmount)
	t.FeedMilli = FloatToMilli(t.Feed)
	t.CutDepthMilli = FloatToMilli(t.CutDepth)
}

// NCFileInput 是写入 nc_file 表所需的字段，由 ncstore 产出。
// 用它做参数是为了让 repo 层不依赖文件存储层的实现。
type NCFileInput struct {
	SHA256       string
	SizeBytes    int64
	OriginalName string
	RelPath      string
	// Encoding 取值 'utf-8' 或 'gbk'。留空时写库为默认值，
	// 读取时再用完整内容检测并回填（见 ncstore.DetectEncoding）。
	Encoding string
}

// ApplyMilli 把工序的落库整数填进对外的十进制字段。查询后必须调用。
func (o *Operation) ApplyMilli() {
	o.ZHeight = MilliToFloat(o.ZHeightMilli)
}

// FillMilli 把工序的对外十进制字段换算成落库整数。写库前必须调用。
func (o *Operation) FillMilli() {
	o.ZHeightMilli = FloatToMilli(o.ZHeight)
}
