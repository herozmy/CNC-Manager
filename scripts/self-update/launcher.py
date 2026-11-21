#!/usr/bin/python3
"""Unprivileged container entrypoint. The child serves HTTP; PID 1 owns updates."""
import fcntl
import hashlib
import http.client
import json
import logging
import os
from pathlib import Path
import platform
import queue
import re
import shutil
import signal
import socket
import socketserver
import subprocess
import sys
import tempfile
import threading
import time
import urllib.request

IMAGE_ID = Path("/usr/local/lib/cnc-image-id")
BUNDLED = Path("/usr/local/lib/cnc-manager-bundled")
DATA = Path(os.environ.get("CNC_DATA_DIR", "/data")).resolve()
STORE = DATA / ".cnc-updates"
SOCKET = "/tmp/cnc-update.sock"
TAG = re.compile(r"v(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})")
API = "https://api.github.com/repos/herozmy/CNC-Manager/releases/latest"
RELEASES = "https://github.com/herozmy/CNC-Manager/releases/download"
jobs = queue.Queue()
lock = threading.Lock()
stopping = threading.Event()
busy = False
child = None


def version(executable):
    return "v" + subprocess.check_output([executable, "--version"], text=True, timeout=10).strip()


def number(tag):
    match = TAG.fullmatch(tag)
    return tuple(map(int, match.groups())) if match else ()


def atomic(path, value):
    with tempfile.NamedTemporaryFile(dir=path.parent, delete=False) as f:
        try:
            f.write(json.dumps(value).encode())
            f.flush()
            os.fsync(f.fileno())
            os.replace(f.name, path)
        finally:
            Path(f.name).unlink(missing_ok=True)


def read(name, default):
    path = STORE / name
    return json.loads(path.read_bytes()) if path.exists() else default


def state(phase, target="", message=""):
    value = dict(phase=phase, target=target, message=message)
    atomic(STORE / "status.json", value)
    return value


def same_image():
    return read("image.json", None) == IMAGE_ID.read_text().strip()


def activate_image():
    if same_image():
        return
    # Write identity last so an interrupted first boot repeats the reset.
    atomic(STORE / "active.json", "")
    (STORE / "trial.json").unlink(missing_ok=True)
    state("idle")
    atomic(STORE / "image.json", IMAGE_ID.read_text().strip())


def installed():
    base = version(BUNDLED)
    if not same_image():
        return BUNDLED, base
    tag = read("active.json", "")
    if isinstance(tag, str) and number(tag) > number(base):
        path = STORE / tag
        try:
            if path.is_file() and version(path) == tag:
                return path, tag
        except (OSError, subprocess.SubprocessError):
            logging.exception("Cannot use stored executable %s", tag)
    return BUNDLED, base


def download(tag):
    with urllib.request.urlopen(API, timeout=20) as r:
        release = json.loads(r.read(1024 * 1024))
    if release.get("tag_name") != tag or release.get("draft") is not False or release.get("prerelease") is not False:
        raise ValueError("最新正式版本已变化，请重新检查")
    asset = f"cnc-manager-{tag}-linux-amd64"
    base = f"{RELEASES}/{tag}/"
    with urllib.request.urlopen(base + "SHA256SUMS", timeout=20) as r:
        sums = [line.split() for line in r.read(65536).decode("ascii").splitlines()]
    hashes = [fields[0] for fields in sums if len(fields) == 2 and fields[1].lstrip("*") == asset]
    if len(hashes) != 1 or not re.fullmatch("[a-fA-F0-9]{64}", hashes[0]):
        raise ValueError("正式版校验清单无效")
    with tempfile.TemporaryDirectory(dir=STORE) as tmp:
        path = Path(tmp) / "candidate"
        digest = hashlib.sha256()
        with urllib.request.urlopen(base + asset, timeout=30) as r, path.open("wb") as f:
            while chunk := r.read(1024 * 1024):
                if stopping.is_set() or f.tell() + len(chunk) > 256 * 1024 * 1024:
                    raise ValueError("下载中断或文件超出大小限制")
                f.write(chunk)
                digest.update(chunk)
            f.flush()
            os.fsync(f.fileno())
        if digest.hexdigest() != hashes[0].lower():
            raise ValueError("下载文件的 SHA-256 校验失败")
        path.chmod(0o700)
        if version(path) != tag:
            raise ValueError("下载文件的程序版本与标签不符")
        path.replace(STORE / tag)
    return STORE / tag


def launch(path, **settings):
    return subprocess.Popen([path], env={**os.environ, "CNC_UPDATE_SOCKET": SOCKET, **settings})


def stop(proc):
    if proc is not None and proc.poll() is None:
        proc.terminate()
        try:
            proc.wait(timeout=15)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait()


def ready(proc, addr, tag):
    host, port = addr.rsplit(":", 1)
    host = host.strip("[]")
    host = {"": "127.0.0.1", "0.0.0.0": "127.0.0.1", "::": "::1"}.get(host, host)
    for _ in range(80):
        if proc.poll() is not None or stopping.is_set():
            break
        conn = http.client.HTTPConnection(host, int(port), timeout=1)
        try:
            conn.request("GET", "/api/meta")
            if json.load(conn.getresponse()).get("version") == tag[1:]:
                return
        except (OSError, ValueError, http.client.HTTPException):
            pass
        finally:
            conn.close()
        time.sleep(0.25)
    raise ValueError("新版启动检查失败")


def copy_data(dst):
    shutil.copytree(DATA, dst, symlinks=True,
                    ignore=lambda parent, names: [STORE.name] if Path(parent) == DATA else [])


def rollback():
    """Resume an interrupted rollback without deleting either version's data."""
    plan = read("rollback.json", None)
    if not plan:
        return
    folder = plan["folder"]
    if not folder.startswith("backup-") or Path(folder).name != folder or not number(plan["previous"]):
        raise ValueError("恢复记录无效")
    backup = STORE / folder / "data"
    failed = backup.parent / "failed-data"
    failed.mkdir(exist_ok=True)
    if plan["phase"] == "retain":
        for path in DATA.iterdir():
            if path != STORE:
                path.rename(failed / path.name)
        plan["phase"] = "restore"
        atomic(STORE / "rollback.json", plan)
    for path in backup.iterdir():
        path.rename(DATA / path.name)
    atomic(STORE / "active.json", plan["previous"])
    (STORE / "trial.json").unlink(missing_ok=True)
    (STORE / "rollback.json").unlink()


def upgrade(target, previous, previous_tag):
    global child
    if not number(previous_tag) or number(target) <= number(previous_tag):
        raise ValueError("只支持安装更高的正式版本")
    state("downloading", target, "正在下载并校验程序")
    candidate = download(target)
    state("installing", target, "正在备份数据并重启程序")
    if stopping.is_set():
        return
    backup = Path(tempfile.mkdtemp(prefix="backup-" + previous_tag + "-", dir=STORE)) / "data"
    stop(child)
    live_trial = False
    backup_complete = False
    try:
        copy_data(backup)
        backup_complete = True
        with tempfile.TemporaryDirectory(dir=STORE) as tmp:
            probe_data = Path(tmp) / "data"
            shutil.copytree(backup, probe_data, symlinks=True)
            with socket.socket() as sock:
                sock.bind(("127.0.0.1", 0))
                addr = f"127.0.0.1:{sock.getsockname()[1]}"
            probe = launch(candidate, CNC_DATA_DIR=str(probe_data), CNC_ADDR=addr, CNC_UPDATE_SOCKET="")
            try:
                ready(probe, addr, target)
            finally:
                stop(probe)
        if stopping.is_set():
            return
        # Keep the candidate selected across abrupt container restarts once it can migrate live data.
        atomic(STORE / "trial.json", {"target": target, "previous": previous_tag, "folder": backup.parent.name})
        atomic(STORE / "active.json", target)
        live_trial = True
        child = launch(candidate)
        ready(child, os.environ.get("CNC_ADDR", "0.0.0.0:8080"), target)
        state("done", target, "升级完成")
        (STORE / "trial.json").unlink()
    except Exception:
        if not backup_complete:
            shutil.rmtree(backup.parent)
        if not stopping.is_set():
            if live_trial:
                stop(child)
                atomic(STORE / "rollback.json", {
                    "folder": backup.parent.name, "previous": previous_tag, "phase": "retain",
                })
                rollback()
            child = launch(previous)
        raise


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        global busy
        self.connection.settimeout(2)
        cmd = self.rfile.readline(64).decode("ascii").strip()
        with lock:
            if cmd == "status":
                result = read("status.json", {"phase": "idle"})
            elif busy or not TAG.fullmatch(cmd):
                result = {"phase": "rejected", "message": "更新进行中或目标版本无效"}
            else:
                result = state("checking", cmd, "正在检查正式版本")
                busy = True
                jobs.put(cmd)
        self.wfile.write(json.dumps(result).encode() + b"\n")


def main():
    global child, busy
    if platform.machine() not in ("x86_64", "amd64"):
        os.execv(BUNDLED, [str(BUNDLED), *sys.argv[1:]])
    if len(sys.argv) > 1:
        path, _ = installed()
        os.execv(path, [str(path), *sys.argv[1:]])
    STORE.mkdir(mode=0o700, parents=True, exist_ok=True)
    # A second container using this volume must not enter the stop/backup/switch window.
    volume_lock = (STORE / "launcher.lock").open("a")
    fcntl.flock(volume_lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    rollback()
    activate_image()
    path, tag = installed()
    last = read("status.json", {})
    if last.get("phase") in ("checking", "downloading", "installing"):
        state("failed", last.get("target", ""), "上次更新被中断，请核对当前版本")
    Path(SOCKET).unlink(missing_ok=True)
    with socketserver.UnixStreamServer(SOCKET, Handler) as server:
        os.chmod(SOCKET, 0o600)
        for sig in (signal.SIGTERM, signal.SIGINT):
            signal.signal(sig, lambda *_: stopping.set())
        threading.Thread(target=server.serve_forever, daemon=True).start()
        child = launch(path)
        try:
            trial = read("trial.json", None)
            if trial and trial["target"] == tag:
                try:
                    ready(child, os.environ.get("CNC_ADDR", "0.0.0.0:8080"), tag)
                except ValueError:
                    if stopping.is_set():
                        return 0
                    stop(child)
                    atomic(STORE / "rollback.json", {**trial, "phase": "retain"})
                    rollback()
                    path, tag = installed()
                    child = launch(path)
                    state("failed", trial["target"], "新版启动失败，已恢复升级前的数据和程序")
                else:
                    state("done", tag, "升级完成")
            (STORE / "trial.json").unlink(missing_ok=True)
            while not stopping.is_set() and child.poll() is None:
                try:
                    target = jobs.get(timeout=0.25)
                except queue.Empty:
                    continue
                try:
                    upgrade(target, path, tag)
                    path, tag = installed()
                except Exception as exc:
                    logging.exception("Application upgrade failed")
                    state("failed", target, str(exc))
                finally:
                    with lock:
                        busy = False
        finally:
            stop(child)
        return 0 if stopping.is_set() else 1


if __name__ == "__main__":
    sys.exit(main())
