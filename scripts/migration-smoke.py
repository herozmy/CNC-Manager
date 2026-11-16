#!/usr/bin/env python3
"""Verify v1 migration and runtime downloads with disposable native/Docker data."""
import argparse
import base64
import hashlib
import http.cookiejar
import json
import os
import pathlib
import shutil
import socket
import sqlite3
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
import uuid

PASSWORD = "correct horse"  # Public test credential; no production account.
CONTENTS = [b"O1000\nG00 X0\nM30\n", "(精车)\nM30\n".encode("gbk"), b"O3000\nM30\n"]
EXTENSIONS = [".NC", ".tap", ".txt"]
# Independent v1 AES-GCM fixtures, followed by a legacy plaintext file.
ENCRYPTED = [
    "Q05DQ05DMDEIfkOlrC/BLImGR0YISt2ymGfgvd+CIGDXAuIOjhiXJ2VQb03/vujCzaMvwnA=",
    "Q05DQ05DMDFnQw0fHSo4tkqz2VzPJVZ5E7JJCiz3F/PdLDh5jlZBzPjk36BZWKQ=",
]


def hashes(root):
    return {str(f.relative_to(root)): hashlib.sha256(f.read_bytes()).hexdigest()
            for f in root.rglob("*") if f.is_file()}


def seed(root):
    root.mkdir(mode=0o755)
    conn = sqlite3.connect(root / "cnccool.db")
    conn.execute("CREATE TABLE schema_migration(version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)")
    ts = "2026-01-02T03:04:05Z"
    schemas = pathlib.Path(__file__).resolve().parent.parent / "backend/internal/migratev1/testdata/v1schema"
    files = sorted(schemas.glob("*.sql"))
    assert len(files) == 10
    for f in files:
        conn.executescript(f.read_text())
        conn.execute("INSERT INTO schema_migration VALUES (?, ?)", (f.stem, ts))

    def insert(table, **values):
        conn.execute("INSERT INTO " + table + "(" + ",".join(values) + ") VALUES (" +
                     ",".join("?" for _ in values) + ")", list(values.values()))

    timestamps = dict(created_at=ts, updated_at=ts)
    insert("drawing", id=7, drawing_no="D-7", name="迁移样件", **timestamps)
    insert("operation", id=11, drawing_id=7, op_no=10, z_height_milli=25500, sort_order=2, **timestamps)
    insert("nc_program", id=21, operation_id=11, program_no="O1000", current_version_id=102, **timestamps)
    for i, plain in enumerate(CONTENTS):
        sha = hashlib.sha256(plain).hexdigest()
        rel = sha[:2] + "/" + sha[2:4] + "/" + sha + EXTENSIONS[i]
        f = root / "nc" / rel
        f.parent.mkdir(parents=True, exist_ok=True)
        f.write_bytes(base64.b64decode(ENCRYPTED[i]) if i < 2 else plain)
        insert("nc_file", id=i+1, sha256=sha, size_bytes=len(plain), original_name="legacy" + EXTENSIONS[i],
               rel_path=rel, file_encoding="gbk" if i == 1 else "utf-8", created_at=ts)
        insert("nc_version", id=101+i, program_id=21, version_no=i+1, file_id=i+1,
               file_name="v%d" % (i+1) + EXTENSIONS[i] if i < 2 else "", created_at=ts)
    insert("program_tool", id=31, program_id=21, version_id=102, seq=1, tool_no="T05",
           offset_no="D05", tool_dia_milli=12500, comp_amount_milli=150,
           custom_params_json='[{"name":"x","value":"1"}]')
    ph = "pbkdf2-sha256$120000$MDEyMzQ1Njc4OWFiY2RlZg$1FcR4VSgaaoV/en0r0RXZLFpUnLvdIsWsficFQGt/Ww"
    insert("app_user", id=1, username="migration-test", display_name="隔离迁移测试", password_hash=ph, **timestamps)
    insert("user_session", id=1, user_id=1, token_hash="old-session", expires_at="2099-01-01T00:00:00Z", created_at=ts)
    conn.commit()
    conn.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image")
    parser.add_argument("--binary")
    parser.add_argument("--migrator")
    parser.add_argument("--version", required=True)
    parser.add_argument("--report", required=True)
    args = parser.parse_args()
    assert bool(args.image) != bool(args.binary), "choose Docker image or native binary"
    assert args.image or args.migrator, "native migration requires --migrator"
    report = pathlib.Path(args.report).resolve()
    report.parent.mkdir(parents=True, exist_ok=True)
    checks = []
    result = {"image": args.image, "version": args.version, "checks": checks, "passed": False}
    name = "cnc-migration-smoke-" + uuid.uuid4().hex[:12]
    process = None
    created = False
    with tempfile.TemporaryDirectory(prefix="cnc-v1-migration-") as directory:
        root = pathlib.Path(directory)
        source, broken, target = root / "v1", root / "broken-v1", root / "v2"
        seed(source)
        snapshot = hashes(source)
        shutil.copytree(source, broken)
        next(f for f in (broken / "nc").rglob("*") if f.is_file()).unlink()
        target.mkdir()
        if args.image:
            os.chown(target, 1000, 1000)

        def migrate(old):
            if args.image:
                command = ["docker", "run", "--rm", "--network=none", "--cap-drop=ALL",
                           "--security-opt=no-new-privileges:true", "--entrypoint", "migrate-v1",
                           "--mount", "type=bind,src=" + str(old) + ",dst=/old,readonly",
                           "--mount", "type=bind,src=" + str(target) + ",dst=/new", args.image,
                           "-from", "/old", "-to", "/new"]
            else:
                command = [args.migrator, "-from", str(old), "-to", str(target)]
            return subprocess.run(command, capture_output=True, text=True, timeout=60)

        try:
            failure = migrate(broken)
            assert failure.returncode != 0 and "文件缺失" in failure.stderr, failure
            assert target.is_dir() and not list(target.iterdir())
            checks.append("missing-nc-cleans-existing-mount")
            success = migrate(source)
            assert success.returncode == 0, success.stderr
            assert "登录会话未导入（1 个）" in success.stdout
            checks.append("retry-migrates-v1")
            assert hashes(source) == snapshot
            checks.append("source-unchanged")
            migrated = hashes(target)
            refusal = migrate(source)
            assert refusal.returncode != 0 and "非空" in refusal.stderr
            assert hashes(target) == migrated
            checks.append("refuses-existing-v2-without-changes")
            conn = sqlite3.connect(target / "cnc.db")
            assert conn.execute("PRAGMA integrity_check").fetchone() == ("ok",)
            assert not conn.execute("PRAGMA foreign_key_check").fetchall()
            assert conn.execute("SELECT count(*) FROM user_session").fetchone()[0] == 0
            assert conn.execute("SELECT z_height_milli,sort_order FROM operation WHERE id=11").fetchone() == (25500, 2)
            conn.close()
            checks.extend(["database-integrity-and-relationships", "old-sessions-skipped"])
            log = open(report.with_suffix(".log"), "w")
            try:
                if args.image:
                    subprocess.run(["docker", "run", "-d", "--name", name, "--cap-drop=ALL",
                                    "--security-opt=no-new-privileges:true", "-p", "127.0.0.1::8080",
                                    "--mount", "type=bind,src=" + str(target) + ",dst=/data", args.image],
                                   check=True, stdout=subprocess.DEVNULL)
                    created = True
                    binding = subprocess.check_output(["docker", "port", name, "8080/tcp"], text=True).strip()
                else:
                    with socket.socket() as sock:
                        sock.bind(("127.0.0.1", 0))
                        binding = "127.0.0.1:" + str(sock.getsockname()[1])
                    process = subprocess.Popen([args.binary], env=dict(os.environ, CNC_ADDR=binding, CNC_DATA_DIR=str(target)), stdout=log, stderr=log)
                base = "http://" + binding
                client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))

                def request(path, body=None):
                    payload = json.dumps(body).encode() if body is not None else None
                    with client.open(urllib.request.Request(base + path, data=payload,
                                     headers={"Origin": base, "Content-Type": "application/json"}), timeout=10) as response:
                        raw = response.read()
                        return json.loads(raw) if "application/json" in response.headers.get("Content-Type", "") else raw

                for _ in range(100):
                    try:
                        assert request("/api/meta")["version"] == args.version
                        break
                    except OSError:
                        time.sleep(.1)
                else:
                    raise AssertionError("migrated service did not start")
                user = request("/api/auth/login", {"username": "migration-test", "password": PASSWORD})
                assert user["role"] == "admin"
                checks.append("v1-password-login")
                drawings = request("/api/drawings?ownerUserId=0")["items"]
                assert len(drawings) == 1 and drawings[0]["id"] == 7 and drawings[0]["ownerUserId"] == 0
                assert request("/api/programs/21")["currentVersionId"] == 102
                checks.append("unassigned-data-and-current-version")
                tools = request("/api/versions/102/tools")["items"]
                assert tools[0]["toolDia"] == 12.5 and tools[0]["compAmount"] == .15
                assert tools[0]["customParams"] == [{"name": "x", "value": "1"}]
                checks.append("tool-values-and-custom-parameters")
                for i, plain in enumerate(CONTENTS):
                    version = str(101+i)
                    assert request("/api/versions/" + version + "/download") == plain
                    content = request("/api/versions/" + version + "/content")
                    assert content["content"] == plain.decode("gbk" if i == 1 else "utf-8")
                    if i == 2:
                        assert content["fileName"] == "legacy" + EXTENSIONS[i]
                checks.extend(["encrypted-utf8-and-gbk-downloads", "plaintext-and-filename-fallback"])
                result["passed"] = True
            finally:
                if process:
                    process.terminate()
                    process.wait(timeout=15)
                if created:
                    logs = subprocess.run(["docker", "logs", name], capture_output=True, text=True)
                    log.write(logs.stdout + logs.stderr)
                log.close()
        except Exception as error:
            result["error"] = str(error)
            raise
        finally:
            if created:
                subprocess.run(["docker", "rm", "-f", name], check=True, stdout=subprocess.DEVNULL)
            report.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
