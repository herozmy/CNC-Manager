package ncstore

import "errors"

// 文件存储层的领域错误。用 errors.Is 判断，便于 HTTP 层映射成 4xx 而不是 500。
var (
	// ErrTooLarge 表示上传文件超过配置的大小上限。
	ErrTooLarge = errors.New("文件过大")
	// ErrEmpty 表示上传文件内容为空。
	ErrEmpty = errors.New("文件内容为空")
)
