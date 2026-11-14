package domain

// Version 是程序的一个版本。文件名属于版本，同一内容可被多个版本共享。
type Version struct {
	ID         int64  `db:"id" json:"id"`
	ProgramID  int64  `db:"program_id" json:"programId"`
	VersionNo  int    `db:"version_no" json:"versionNo"`
	SHA256     string `db:"sha256" json:"sha256"`
	FileName   string `db:"file_name" json:"fileName"`
	FileSize   int64  `db:"file_size" json:"fileSize"`
	Encoding   string `db:"encoding" json:"encoding"`
	ChangeNote string `db:"change_note" json:"changeNote"`
	CreatedBy  string `db:"created_by" json:"createdBy"`
	CreatedAt  string `db:"created_at" json:"createdAt"`
	IsCurrent  bool   `db:"is_current" json:"isCurrent"`
}

// BlobInput 是写入 nc_blob 所需字段，由 ncstore 产出。
// 用它做参数，让 repo 不依赖文件存储层的实现。
type BlobInput struct {
	SHA256    string
	SizeBytes int64
	Encoding  string
	RelPath   string
}

// VersionFile 是读取版本内容所需的信息。
type VersionFile struct {
	SHA256    string `db:"sha256"`
	VersionID int64  `db:"id"`
	ProgramID int64  `db:"program_id"`
	VersionNo int    `db:"version_no"`
	FileName  string `db:"file_name"`
	RelPath   string `db:"rel_path"`
	Encoding  string `db:"encoding"`
	IsCurrent bool   `db:"is_current"`
}

// VersionContent 是版本文本，已按 Encoding 解码成 UTF-8 供界面显示与编辑。
// 保存时必须把 Encoding 原样带回，后端才能按同样编码写回。
type VersionContent struct {
	SHA256    string `json:"sha256"`
	VersionID int64  `json:"versionId"`
	ProgramID int64  `json:"programId"`
	VersionNo int    `json:"versionNo"`
	FileName  string `json:"fileName"`
	Encoding  string `json:"encoding"`
	Content   string `json:"content"`
	LineCount int    `json:"lineCount"`
	SizeBytes int    `json:"sizeBytes"`
	IsCurrent bool   `json:"isCurrent"`
}

// ContentInput 是保存程序文本的请求体。
type ContentInput struct {
	ExpectedSHA256 *string `json:"expectedSHA256,omitempty"` // 覆盖时用于乐观并发检查；旧客户端可省略。
	Content        string  `json:"content"`
	Encoding       string  `json:"encoding"`
	FileName       string  `json:"fileName"`
	ChangeNote     string  `json:"changeNote"`
}

// VersionSaveResult 是保存文本的响应；Warning 提示被目标编码存不下而替换的字符。
type VersionSaveResult struct {
	Version
	Warning string `json:"warning,omitempty"`
}

// UploadVersionResult 是上传新版本的响应，附带识别出的刀具调用。
type UploadVersionResult struct {
	Version
	Parse *ParseResult `json:"parse,omitempty"`
}

// ParseTextInput 是"解析一段 NC 文本"的请求体。
type ParseTextInput struct {
	Content string `json:"content"`
}

// SetCurrentInput 是切换当前版本（回滚）的请求体。
type SetCurrentInput struct {
	VersionID int64 `json:"versionId"`
}
