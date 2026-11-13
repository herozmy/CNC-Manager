// Package config 定义数据目录布局。
package config

import "path/filepath"

// Layout 是数据目录内各项数据的位置。整个目录拷走即可迁移全部数据。
type Layout struct {
	Root    string
	DBPath  string // SQLite 数据库
	NCDir   string // NC 文件库
	KeyPath string // NC 文件加密密钥，必须与数据一并备份
}

// NewLayout 根据数据根目录计算各路径。
func NewLayout(root string) Layout {
	return Layout{
		Root:    root,
		DBPath:  filepath.Join(root, "cnc.db"),
		NCDir:   filepath.Join(root, "nc"),
		KeyPath: filepath.Join(root, "nc.key"),
	}
}
