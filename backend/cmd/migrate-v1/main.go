// migrate-v1 把 v1（cnccool）数据目录无损导入 v2 数据目录。
//
//	migrate-v1 -from <v1 数据目录> -to <v2 数据目录>
//
// v1 数据目录含 cnccool.db 与 nc/；旧数据只读，不会被修改。
// 导入失败会整体回滚并清理新数据；原有空目录（含 Docker 绑定挂载）保留。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"cnc-manager/internal/config"
	"cnc-manager/internal/migratev1"
	"cnc-manager/internal/ncstore"
	"cnc-manager/internal/store"
)

func main() {
	from := flag.String("from", "", "v1 数据目录（含 cnccool.db 与 nc/）")
	to := flag.String("to", "", "v2 数据目录（必须为空或不存在）")
	flag.Parse()
	if *from == "" || *to == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*from, *to); err != nil {
		fmt.Fprintln(os.Stderr, "迁移失败:", err)
		os.Exit(1)
	}
}

func run(from, to string) error {
	if entries, err := os.ReadDir(to); err == nil && len(entries) > 0 {
		return fmt.Errorf("目标目录 %s 非空，拒绝导入", to)
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("读取目标目录失败: %w", err)
	}
	_, existed := os.Stat(to)
	l := config.NewLayout(to)

	rep, err := doImport(from, l)
	if err != nil {
		if existed != nil {
			err = errors.Join(err, os.RemoveAll(to))
		} else {
			// Docker 的绑定挂载根目录不能删除；仅清理本次生成的内容，保留原空目录。
			for _, path := range []string{l.DBPath, l.DBPath + "-wal", l.DBPath + "-shm", l.KeyPath, l.NCDir} {
				err = errors.Join(err, os.RemoveAll(path))
			}
		}
		return err
	}

	fmt.Println("迁移完成：")
	names := make([]string, 0, len(rep.Counts))
	for k := range rep.Counts {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		fmt.Printf("  %-14s %d\n", k, rep.Counts[k])
	}
	if rep.OrphanFiles > 0 {
		fmt.Printf("  未被任何版本引用而未导入的 NC 文件: %d\n", rep.OrphanFiles)
	}
	fmt.Printf("  登录会话未导入（%d 个），用户需重新登录\n", rep.SkippedSession)
	fmt.Printf("新数据目录: %s\n请妥善备份其中的 nc.key，丢失后所有 NC 文件将无法解密。\n", to)
	return nil
}

func doImport(from string, l config.Layout) (*migratev1.Report, error) {
	key, err := ncstore.LoadOrCreateKey(l.KeyPath)
	if err != nil {
		return nil, err
	}
	ns, err := ncstore.New(l.NCDir, 64<<20, key)
	if err != nil {
		return nil, err
	}
	db, err := store.Open(l.DBPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	ctx := context.Background()
	if err := store.Migrate(ctx, db); err != nil {
		return nil, err
	}
	return migratev1.Run(ctx, filepath.Join(from, "cnccool.db"), filepath.Join(from, "nc"), db, ns)
}
