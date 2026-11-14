#!/usr/bin/env python3
"""Interactive, server-side CNC-Manager password recovery (Python 3 stdlib)."""

import argparse
import base64
from contextlib import closing, contextmanager
import datetime
import fcntl
import getpass
import hashlib
import os
from pathlib import Path
import shutil
import signal
import sqlite3
import subprocess
import sys
import uuid
import warnings


class ResetError(Exception):
    pass


def connect(data, mode="ro"):
    path = (Path(data) / "cnc.db").resolve()
    return sqlite3.connect(path.as_uri() + "?mode=" + mode, uri=True, timeout=15)


def accounts(data):
    with closing(connect(data)) as db:
        return db.execute(
            "SELECT id, username, display_name, enabled FROM app_user ORDER BY id"
        ).fetchall()


def select_account(users, username=None, ask=input, output=print):
    enabled = [user for user in users if user[3] == 1]
    if username is not None:
        # Match SQLite NOCASE for the usual ASCII login names; do not casefold
        # non-ASCII text into a different account.
        ascii_lower = lambda value: value.translate(str.maketrans("ABCDEFGHIJKLMNOPQRSTUVWXYZ", "abcdefghijklmnopqrstuvwxyz"))
        selected = [user for user in enabled if ascii_lower(user[1]) == ascii_lower(username.strip())]
        if len(selected) != 1:
            raise ResetError("未找到启用中的登录用户名，请用 --list 查看账号。")
        return selected[0]
    if not enabled:
        raise ResetError("没有可重置的启用账号；本工具不会创建或重新启用账号。")
    if len(enabled) == 1:
        output(f"已选择账号：{enabled[0][1]}（{enabled[0][2]}）")
        return enabled[0]
    for number, user in enumerate(enabled, 1):
        output(f"  {number}. {user[1]}（{user[2]}）")
    while True:
        choice = ask("选择账号编号（输入 0 取消）：").strip()
        if choice == "0":
            return None
        if choice.isascii() and choice.isdigit() and 1 <= int(choice) <= len(enabled):
            return enabled[int(choice) - 1]
        output("请输入列表中的编号。")


def read_password(read_secret=getpass.getpass, output=print):
    while True:
        password = read_secret("新密码（8~128 个字符，输入不显示）：")
        if not 8 <= len(password) <= 128:
            output("密码长度不符合要求，请重新输入。")
            continue
        if password != read_secret("再次输入新密码："):
            output("两次密码不一致，请重新输入。")
            continue
        return password


def hash_password(password):
    if not 8 <= len(password) <= 128:
        raise ResetError("密码应为 8~128 个字符。")
    salt = os.urandom(16)
    key = hashlib.pbkdf2_hmac("sha256", password.encode("utf-8"), salt, 120000, 32)
    encode = lambda value: base64.b64encode(value).decode("ascii").rstrip("=")
    return f"pbkdf2-sha256$120000${encode(salt)}${encode(key)}"


class SystemdService:
    def __init__(self, name, data):
        self.name = name
        self.data = Path(data).resolve()

    def command(self, *args):
        return subprocess.run(
            ["systemctl", *args, self.name], check=True,
            capture_output=True, text=True, timeout=45,
        ).stdout.strip()

    def is_running(self):
        if os.geteuid() != 0:
            raise ResetError("请使用服务器 root 账号运行。")
        if self.command("show", "--property=LoadState", "--value") != "loaded":
            raise ResetError(f"服务 {self.name} 未安装。")
        state = self.command("show", "--property=ActiveState", "--value")
        if state not in ("active", "inactive", "failed"):
            raise ResetError("服务正在切换状态，请稍后重试。")
        # A custom data directory must never stop/reset an unrelated service.
        import shlex
        env = dict(item.split("=", 1) for item in shlex.split(
            self.command("show", "--property=Environment", "--value")
        ) if "=" in item)
        if "CNC_DATA_DIR" not in env or Path(env["CNC_DATA_DIR"]).resolve() != self.data:
            raise ResetError("服务数据目录与指定目录不一致，已停止操作。")
        return state == "active"

    def stop(self):
        self.command("stop")

    def start(self):
        self.command("start")
        self.command("is-active", "--quiet")


@contextmanager
def maintenance_lock(directory):
    directory.mkdir(parents=True, exist_ok=True, mode=0o700)
    fd = os.open(str(directory / ".password-reset.lock"), os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    try:
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as error:
            raise ResetError("另一个密码重置程序正在运行，请稍后重试。") from error
        yield
    finally:
        os.close(fd)


@contextmanager
def restore_on_signal():
    def interrupted(signum, frame):
        raise InterruptedError("操作中断，正在恢复服务。")
    previous = {sig: signal.signal(sig, interrupted) for sig in (signal.SIGTERM, signal.SIGHUP)}
    try:
        yield
    finally:
        for sig, handler in previous.items():
            signal.signal(sig, handler)


def reset_account(data, account, password, service=None, output=print):
    """service=None is for an explicitly offline data directory."""
    data = Path(data).resolve()
    encoded = hash_password(password)
    del password
    backup_root = data.parent / "backups"
    with maintenance_lock(backup_root):
        running = service.is_running() if service else False
        owner = (data / "cnc.db").stat()
        backup = backup_root / ("pre-password-reset-" + datetime.datetime.now().strftime("%Y%m%d-%H%M%S") + "-" + uuid.uuid4().hex[:8])
        changed = False
        try:
            if running:
                output("正在暂停服务并备份数据…")
                service.stop()
            else:
                output("正在备份离线数据…")
            backup.mkdir(mode=0o700)
            shutil.copytree(data, backup / "data", symlinks=True)
            output(f"备份已保存：{backup}")
            with closing(connect(data, "rw")) as db:
                db.execute("PRAGMA foreign_keys=ON")
                try:
                    db.execute("BEGIN IMMEDIATE")
                    now = datetime.datetime.now(datetime.timezone.utc).isoformat(timespec="seconds").replace("+00:00", "Z")
                    result = db.execute(
                        "UPDATE app_user SET password_hash = ?, updated_at = ? "
                        "WHERE id = ? AND username = ? AND enabled = 1",
                        (encoded, now, account[0], account[1]),
                    )
                    if result.rowcount != 1:
                        raise ResetError("账号状态已变化，没有重置密码。")
                    revoked = db.execute("DELETE FROM user_session WHERE user_id = ?", (account[0],)).rowcount
                    db.execute(
                        "INSERT INTO audit_log (action, entity_type, entity_id, detail, actor, created_at) "
                        "VALUES ('reset_password', 'app_user', ?, ?, 'server-admin', ?)",
                        (account[0], f"管理员重置密码，撤销 {revoked} 个会话", now),
                    )
                    db.commit()
                    changed = True
                except BaseException:
                    db.rollback()
                    raise
        finally:
            try:
                for name in ("cnc.db", "cnc.db-wal", "cnc.db-shm"):
                    path = data / name
                    if path.exists() and (path.stat().st_uid, path.stat().st_gid) != (owner.st_uid, owner.st_gid):
                        os.chown(path, owner.st_uid, owner.st_gid)
            finally:
                if running:
                    try:
                        service.start()
                    except Exception as error:
                        state = "密码已重置" if changed else "密码未修改"
                        raise ResetError(f"{state}，但服务恢复失败。请检查 systemctl status {service.name}；备份：{backup}") from error
        output(f"完成！账号 {account[1]} 的密码已重置，已清除 {revoked} 个旧登录。")
        output("服务已恢复。" if running else "数据目录保持离线状态。")
        return {"username": account[1], "backup": str(backup), "revokedSessions": revoked, "serviceRestored": running}


def main(argv=None):
    parser = argparse.ArgumentParser(description="CNC-Manager 交互式密码重置：选择账号、输入新密码，自动备份并恢复服务。")
    parser.add_argument("--user", help="指定登录用户名；省略则选择账号")
    parser.add_argument("--list", action="store_true", help="只列出账号，不修改数据")
    parser.add_argument("--data-dir", type=Path, default=Path("/opt/cnc-manager-v2/data"), help="已有数据目录")
    parser.add_argument("--service", default="cnc-manager-v2.service", help="管理的 systemd 服务名称")
    parser.add_argument("--no-service", action="store_true", help="仅用于已停止服务的离线数据目录，由调用方保证没有其他进程写入")
    args = parser.parse_args(argv)
    try:
        users = accounts(args.data_dir)
        if args.list:
            for user in users:
                print(f"{user[1]}（{user[2]}） · {'启用' if user[3] == 1 else '停用'}")
            return 0
        if not sys.stdin.isatty():
            raise ResetError("请在交互式终端运行；远程连接请使用 ssh -t。")
        manager = None if args.no_service else SystemdService(args.service, args.data_dir)
        if manager:
            manager.is_running()
        warnings.simplefilter("error", getpass.GetPassWarning)
        print("CNC-Manager 密码重置\n无需原应用密码。按 Ctrl+C 可取消。")
        account = select_account(users, args.user)
        if account is None:
            print("已取消，没有修改密码。")
            return 0
        print(f"将重置：{account[1]}；原业务数据保留。")
        password = read_password()
        with restore_on_signal():
            reset_account(args.data_dir, account, password, manager)
        print("请使用新密码重新登录 CNC-Manager。")
        return 0
    except (KeyboardInterrupt, EOFError, InterruptedError):
        print("\n操作已中断，请以终端中是否出现“完成”及服务状态确认结果。", file=sys.stderr)
        return 130
    except (ResetError, sqlite3.Error, OSError, subprocess.SubprocessError, getpass.GetPassWarning) as error:
        print(f"操作未完成：{error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
