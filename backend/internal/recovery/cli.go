package recovery

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

func Run(ctx context.Context, args []string, in *os.File, out io.Writer) error {
	flags := flag.NewFlagSet("cnc-manager reset-password", flag.ContinueOnError)
	flags.SetOutput(out)
	dataDefault := os.Getenv("CNC_DATA_DIR")
	if dataDefault == "" {
		dataDefault = "/opt/cnc-manager-v2/data"
	}
	data := flags.String("data-dir", dataDefault, "已有数据目录（也可通过 CNC_DATA_DIR 设置）")
	serviceName := flags.String("service", "cnc-manager-v2.service", "管理的 systemd 服务名称")
	user := flags.String("user", "", "指定登录用户名；省略则选择账号")
	list := flags.Bool("list", false, "只列出账号，不重置密码")
	offline := flags.Bool("no-service", false, "仅用于已停止服务的离线目录；旧版服务须手动确认停止")
	flags.Usage = func() {
		fmt.Fprintln(out, "用法：cnc-manager reset-password [选项]\n选择账号、隐藏输入新密码、备份数据并恢复服务；不需要 Python。")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("不接受位置参数或命令行密码；请用 --help 查看用法")
	}
	users, err := Accounts(ctx, *data)
	if err != nil {
		return err
	}
	if *list {
		for _, a := range users {
			state := "停用"
			if a.Enabled {
				state = "启用"
			}
			fmt.Fprintf(out, "%s（%s） · %s\n", a.Username, a.DisplayName, state)
		}
		return nil
	}
	fd := int(in.Fd())
	if !term.IsTerminal(fd) {
		return errors.New("请在交互式终端运行；远程连接请使用 ssh -t")
	}
	var manager Service
	if !*offline {
		manager = Systemd{Name: *serviceName, Data: *data}
		if _, err := manager.Running(ctx); err != nil {
			return err
		}
	}
	fmt.Fprintln(out, "CNC-Manager 密码重置\n无需原应用密码。按 Ctrl+C 可取消。")
	reader := bufio.NewReader(in)
	account, err := selectAccount(users, *user, func(prompt string) (string, error) {
		fmt.Fprint(out, prompt)
		return readWithContext(ctx, func() (string, error) { return reader.ReadString('\n') })
	}, out)
	if err != nil {
		return err
	}
	if account == nil {
		fmt.Fprintln(out, "已取消，没有修改密码。")
		return nil
	}
	fmt.Fprintf(out, "将重置：%s；原业务数据保留。\n", account.Username)
	password, err := readPassword(func(prompt string) (string, error) {
		state, err := term.GetState(fd)
		if err != nil {
			return "", err
		}
		defer term.Restore(fd, state)
		fmt.Fprint(out, prompt)
		value, err := readWithContext(ctx, func() (string, error) { b, e := term.ReadPassword(fd); defer clear(b); return string(b), e })
		fmt.Fprintln(out)
		return value, err
	}, out)
	if err != nil {
		return err
	}
	result, err := Reset(ctx, *data, *account, password, manager, out)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "完成！账号 %s 的密码已重置，已清除 %d 个旧登录。\n", account.Username, result.Revoked)
	if result.ServiceRestored {
		fmt.Fprintln(out, "服务已恢复。")
	} else {
		fmt.Fprintln(out, "数据目录保持离线状态。")
	}
	fmt.Fprintln(out, "请使用新密码重新登录 CNC-Manager。")
	return nil
}

func readWithContext(ctx context.Context, read func() (string, error)) (string, error) {
	type response struct {
		value string
		err   error
	}
	ch := make(chan response, 1)
	go func() { value, err := read(); ch <- response{value, err} }()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-ch:
		return r.value, r.err
	}
}

func asciiLower(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, s)
}

func selectAccount(users []Account, username string, ask func(string) (string, error), out io.Writer) (*Account, error) {
	var enabled []Account
	for _, a := range users {
		if a.Enabled {
			enabled = append(enabled, a)
		}
		if username != "" && a.Enabled && asciiLower(a.Username) == asciiLower(strings.TrimSpace(username)) {
			return &a, nil
		}
	}
	if username != "" {
		return nil, errors.New("未找到启用中的登录用户名，请用 --list 查看账号")
	}
	if len(enabled) == 0 {
		return nil, errors.New("没有可重置的启用账号；不会创建或重新启用账号")
	}
	if len(enabled) == 1 {
		return &enabled[0], nil
	}
	for i, a := range enabled {
		fmt.Fprintf(out, "  %d. %s（%s）\n", i+1, a.Username, a.DisplayName)
	}
	for {
		answer, err := ask("选择账号编号（输入 0 取消）：")
		if err != nil {
			return nil, err
		}
		n, err := strconv.Atoi(strings.TrimSpace(answer))
		if err == nil && n == 0 {
			return nil, nil
		}
		if err == nil && n > 0 && n <= len(enabled) {
			return &enabled[n-1], nil
		}
		fmt.Fprintln(out, "请输入列表中的编号。")
	}
}

func readPassword(secret func(string) (string, error), out io.Writer) (string, error) {
	for {
		password, err := secret("新密码（8~128 个字符，输入不显示）：")
		if err != nil {
			return "", err
		}
		if n := utf8.RuneCountInString(password); n < 8 || n > 128 {
			fmt.Fprintln(out, "密码长度不符合要求，请重新输入。")
			continue
		}
		confirm, err := secret("再次输入新密码：")
		if err != nil {
			return "", err
		}
		if password != confirm {
			fmt.Fprintln(out, "两次密码不一致，请重新输入。")
			continue
		}
		return password, nil
	}
}
