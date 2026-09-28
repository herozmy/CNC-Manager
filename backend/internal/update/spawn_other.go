//go:build !windows

package update

import "errors"

// SpawnDetached 在非 Windows 上不可用。
//
// 离线自更新是为 Windows 免安装包做的：容器化部署应该走替换镜像，
// 而不是让服务在半路把自己换掉。
func SpawnDetached(string) error {
	return errors.New("离线安装目前只支持 Windows 免安装版")
}
