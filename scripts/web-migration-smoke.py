#!/usr/bin/env python3
"""Exercise v1 ZIP web preview/import and v2 recovery on disposable data."""
import argparse
import hashlib
import http.cookiejar
import importlib.util
import io
import json
import os
import pathlib
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
import uuid
import zipfile

spec = importlib.util.spec_from_file_location("migration_fixture", pathlib.Path(__file__).with_name("migration-smoke.py"))
fixture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fixture)


def v1zip(source):
    out = io.BytesIO()
    with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as z:
        z.writestr("manifest.json", json.dumps({"format": 1, "version": "1.2.13", "createdAt": "2026-01-02T03:04:05Z"}))
        z.write(source / "cnccool.db", "data/cnccool.db")
        for f in sorted((source / "nc").rglob("*")):
            if f.is_file():
                z.write(f, "data/" + f.relative_to(source).as_posix())
    return out.getvalue()


class Client:
    def __init__(self, base):
        self.base = base
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))

    def request(self, path, body=None, expected=200, content_type="application/json"):
        payload = json.dumps(body).encode() if isinstance(body, dict) else body
        req = urllib.request.Request(self.base + path, data=payload,
                                     headers={"Origin": self.base, "Content-Type": content_type})
        try:
            response = self.opener.open(req, timeout=60)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            raw = response.read()
            assert response.code == expected, (path, response.code, raw)
            return json.loads(raw) if "application/json" in response.headers.get("Content-Type", "") else raw

    def upload(self, path, raw, fields=None, expected=200, filename="backup.zip"):
        boundary = "cnc-web-migration-" + uuid.uuid4().hex
        body = b""
        for key, value in (fields or {}).items():
            body += (f'--{boundary}\r\nContent-Disposition: form-data; name="{key}"\r\n\r\n{value}\r\n').encode()
        body += (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{filename}"\r\n'
                 'Content-Type: application/octet-stream\r\n\r\n').encode()
        body += raw + f"\r\n--{boundary}--\r\n".encode()
        return self.request(path, body, expected, "multipart/form-data; boundary=" + boundary)


def scenario(client, raw, checks):
    original = client.request("/api/auth/setup", {"username": "original", "displayName": "迁移前账号", "password": fixture.PASSWORD})
    drawing = client.request("/api/drawings", {"drawingNo": "CURRENT", "ownerUserId": original["id"]}, 201)
    operation = client.request(f'/api/drawings/{drawing["id"]}/operations', {"opNo": 10}, 201)
    program = client.request(f'/api/operations/{operation["id"]}/programs', {"programNo": "O9999"}, 201)
    current_nc = b"O9999\nM30\n"
    version = client.upload(f'/api/programs/{program["id"]}/versions', current_nc, expected=201, filename="original.nc")
    current_version_id = version["id"]
    with zipfile.ZipFile(io.BytesIO(client.request("/api/backup/export"))) as z:
        key_before = z.read("data/nc.key")
    checks.append("existing-v2-account-and-nc")
    Client(client.base).upload("/api/backup/inspect", raw, expected=401)
    checks.append("anonymous-v1-import-rejected")
    bad = io.BytesIO()
    with zipfile.ZipFile(io.BytesIO(raw)) as source, zipfile.ZipFile(bad, "w") as dest:
        for f in source.infolist():
            if not f.filename.startswith("data/nc/"):
                dest.writestr(f.filename, source.read(f))
    client.upload("/api/backup/inspect", bad.getvalue(), expected=400)
    assert client.request("/api/auth/me")["username"] == "original"
    assert client.request(f'/api/versions/{current_version_id}/download') == current_nc
    checks.append("missing-nc-keeps-original-login-and-file")
    preview = client.upload("/api/backup/inspect", raw)
    assert preview["format"] == "cnc-manager-v1/1" and preview["summary"]["versions"] == 3
    assert preview["sha256"] == hashlib.sha256(raw).hexdigest()
    assert preview["migration"]["skippedSessions"] == 1
    assert client.request("/api/auth/me")["username"] == "original"
    assert client.request(f'/api/versions/{current_version_id}/download') == current_nc
    checks.append("v1-preview-without-live-changes")
    client.upload("/api/backup/import", raw, {"sha256": preview["sha256"]}, expected=400)
    client.upload("/api/backup/import", raw, {"sha256": "changed", "confirmation": "覆盖导入"}, expected=400)
    assert client.request("/api/auth/me")["username"] == "original"
    checks.append("confirmation-and-original-zip-hash-required")
    restored = client.upload("/api/backup/import", raw, {"sha256": preview["sha256"], "confirmation": "覆盖导入"})
    client.request("/api/auth/me", expected=401)
    client.request("/api/auth/login", {"username": "original", "password": fixture.PASSWORD}, expected=401)
    user = client.request("/api/auth/login", {"username": "migration-test", "password": fixture.PASSWORD})
    assert user["role"] == "admin"
    checks.append("v1-password-login-and-old-session-revocation")
    drawings = client.request("/api/drawings?ownerUserId=0")["items"]
    assert len(drawings) == 1 and drawings[0]["id"] == 7 and drawings[0]["ownerUserId"] == 0
    assert client.request("/api/programs/21")["currentVersionId"] == 102
    assert client.request("/api/programs/21")["machineId"] == 3
    tools = client.request("/api/versions/102/tools")["items"]
    assert tools[0]["toolDia"] == 12.5 and tools[0]["compAmount"] == .15
    checks.append("business-current-version-and-tool-parameters")
    for i, plain in enumerate(fixture.CONTENTS):
        vid = str(101+i)
        assert client.request("/api/versions/" + vid + "/download") == plain
        content = client.request("/api/versions/" + vid + "/content")
        assert content["content"] == plain.decode("gbk" if i == 1 else "utf-8")
        assert content["fileName"] == ("v%d" % (i+1) if i < 2 else "legacy") + fixture.EXTENSIONS[i]
    checks.append("encrypted-utf8-gbk-and-plaintext-downloads")
    with zipfile.ZipFile(io.BytesIO(client.request("/api/backup/export"))) as z:
        assert z.read("data/nc.key") == key_before
    checks.append("destination-key-preserved")
    recovery = client.request("/api/backup/recoveries/" + restored["recoveryName"])
    before = client.upload("/api/backup/inspect", recovery)
    assert before["format"] == "cnc-manager-v2/1" and "migration" not in before
    assert before["summary"]["admins"] == ["original"]
    checks.append("automatic-v2-recovery-copy")
    client.upload("/api/backup/import", recovery, {"sha256": before["sha256"], "confirmation": "覆盖导入"})
    client.request("/api/auth/login", {"username": "original", "password": fixture.PASSWORD})
    assert client.request(f'/api/versions/{current_version_id}/download') == current_nc
    assert client.request("/api/drawings")["items"][0]["drawingNo"] == "CURRENT"
    checks.append("recovery-restores-original-account-business-and-nc")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image")
    parser.add_argument("--binary")
    parser.add_argument("--version", required=True)
    parser.add_argument("--report", required=True)
    args = parser.parse_args()
    assert bool(args.image) != bool(args.binary)
    report = pathlib.Path(args.report).resolve()
    report.parent.mkdir(parents=True, exist_ok=True)
    checks = []
    result = {"version": args.version, "image": args.image, "checks": checks, "passed": False}
    name = "cnc-web-migration-" + uuid.uuid4().hex[:12]
    process, created = None, False
    with tempfile.TemporaryDirectory(prefix="cnc-v1-web-") as directory:
        root = pathlib.Path(directory)
        source, data = root / "v1", root / "v2"
        fixture.seed(source)
        raw = v1zip(source)
        data.mkdir()
        log = open(report.with_suffix(".log"), "w")
        try:
            if args.image:
                os.chown(data, 1000, 1000)
                subprocess.run(["docker", "run", "-d", "--name", name, "--cap-drop=ALL",
                                "--security-opt=no-new-privileges:true", "-p", "127.0.0.1::8080", "--mount",
                                "type=bind,src=" + str(data) + ",dst=/data", args.image], check=True, stdout=subprocess.DEVNULL)
                created = True
                binding = subprocess.check_output(["docker", "port", name, "8080/tcp"], text=True).strip()
            else:
                with socket.socket() as sock:
                    sock.bind(("127.0.0.1", 0))
                    binding = "127.0.0.1:" + str(sock.getsockname()[1])
                process = subprocess.Popen([args.binary], env=dict(os.environ, CNC_ADDR=binding, CNC_DATA_DIR=str(data)), stdout=log, stderr=log)
            client = Client("http://" + binding)
            for _ in range(100):
                try:
                    assert client.request("/api/meta")["version"] == args.version
                    break
                except OSError:
                    time.sleep(.1)
            else:
                raise AssertionError("service did not start")
            scenario(client, raw, checks)
            result["passed"] = True
        except Exception as error:
            result["error"] = str(error)
            raise
        finally:
            if process:
                process.terminate()
                process.wait(timeout=15)
            if created:
                logs = subprocess.run(["docker", "logs", name], capture_output=True, text=True)
                log.write(logs.stdout + logs.stderr)
                subprocess.run(["docker", "rm", "-f", name], check=True, stdout=subprocess.DEVNULL)
            log.close()
            report.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
