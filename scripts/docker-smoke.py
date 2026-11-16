#!/usr/bin/env python3
"""Exercise a Docker image with disposable containers and a private data volume."""
import argparse
import hashlib
import http.cookiejar
import io
import json
import pathlib
import re
import subprocess
import time
import urllib.error
import urllib.request
import uuid
import zipfile


def docker(*args):
    return subprocess.check_output(["docker", *args], text=True).strip()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image", required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--report", default=".local/docker-smoke.json")
    args = parser.parse_args()
    name = "cnc-manager-v2-smoke-" + uuid.uuid4().hex[:12]
    volume = name + "-data"
    report = pathlib.Path(args.report)
    report.parent.mkdir(parents=True, exist_ok=True)
    cookies = http.cookiejar.CookieJar()
    client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cookies))
    checks = []
    base = ""

    def request(path, body=None, expected=200, content_type="application/json", method=None):
        payload = json.dumps(body).encode() if isinstance(body, dict) else body
        req = urllib.request.Request(base + path, data=payload, method=method,
                                     headers={"Origin": base, "Content-Type": content_type})
        try:
            response = client.open(req, timeout=10)
        except urllib.error.HTTPError as error:
            response = error
        raw = response.read()
        assert response.code == expected, (path, response.code, raw)
        return raw

    def start():
        nonlocal base
        docker("run", "-d", "--name", name, "--cap-drop=ALL",
               "--security-opt=no-new-privileges:true", "-p", "127.0.0.1::8080",
               "-v", volume + ":/data", args.image)
        port = docker("port", name, "8080/tcp").rsplit(":", 1)[1]
        base = "http://127.0.0.1:" + port
        for _ in range(100):
            try:
                meta = json.loads(request("/api/meta"))
                assert meta["version"] == args.version, meta
                return
            except OSError:
                time.sleep(0.2)
        raise AssertionError("container did not become ready")

    docker("volume", "create", volume)
    result = {"image": args.image, "version": args.version, "checks": checks}
    try:
        assert docker("run", "--rm", args.image, "--version") == args.version
        assert "reset-password" in docker("run", "--rm", args.image, "reset-password", "--help")
        migration_help = subprocess.run(["docker", "run", "--rm", "--entrypoint", "migrate-v1", args.image, "-help"],
                                        capture_output=True, text=True, timeout=20)
        assert migration_help.returncode == 0 and "-from" in migration_help.stdout + migration_help.stderr
        start()
        assert docker("exec", name, "id", "-u") == "1000"
        assert docker("exec", name, "id", "-g") == "1000"
        assert json.loads(request("/api/auth/status"))["setupRequired"]
        html = request("/").decode()
        assert '<div id="root">' in html
        assets = re.findall(r'(?:src|href)="(/assets/[^"]+)"', html)
        assert assets
        for asset in assets:
            assert request(asset)
        request("/api/drawings", expected=401)
        checks.extend(["version", "password-reset-command", "migration-command", "non-root-uid-gid", "embedded-frontend", "fresh-setup", "anonymous-401"])
        user = json.loads(request("/api/auth/setup", {
            "username": "docker-smoke", "displayName": "Docker Smoke",
            "password": "SyntheticDocker123!"}))
        drawing = json.loads(request("/api/drawings", {
            "drawingNo": "DOCKER-SMOKE", "name": "Synthetic part", "ownerUserId": user["id"]}, 201))
        request("/api/drawings", {"drawingNo": ""}, 400)
        operation = json.loads(request(f'/api/drawings/{drawing["id"]}/operations',
                                       {"opNo": 10, "opName": "Docker test"}, 201))
        program = json.loads(request(f'/api/operations/{operation["id"]}/programs',
                                     {"programNo": "O9001", "controller": "FANUC"}, 201))
        content = b"O9001\nT0101\nG00 X1 Z2\nM30\n"
        boundary = "cnc-smoke-" + uuid.uuid4().hex
        body = (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="smoke.nc"\r\n'
                'Content-Type: application/octet-stream\r\n\r\n').encode()
        body += content + f"\r\n--{boundary}--\r\n".encode()
        version = json.loads(request(f'/api/programs/{program["id"]}/versions', body, 201,
                                     "multipart/form-data; boundary=" + boundary))
        assert request(f'/api/versions/{version["id"]}/download') == content
        machine_a = json.loads(request('/api/machines', {'code': 'CNC-A', 'name': 'Machine A'}, 201))
        machine_b = json.loads(request('/api/machines', {'code': 'CNC-B', 'name': 'Machine B'}, 201))
        current = json.loads(request(f'/api/programs/{program["id"]}'))
        current = json.loads(request(f'/api/programs/{program["id"]}', {
            'programNo': 'O9001', 'expectedRevision': current['revision'], 'machineId': machine_a['id']}, method='PUT'))
        sibling = json.loads(request(f'/api/operations/{operation["id"]}/programs', {
            'programNo': 'O9002', 'machineId': machine_b['id']}, 201))
        assert current['machineId'] == machine_a['id'] and sibling['machineId'] == machine_b['id']
        linked = json.loads(request(f'/api/machines/{machine_a["id"]}/drawings?q=O9001'))
        assert linked['total'] == 1 and linked['items'][0]['programCount'] == 1
        assert json.loads(request(f'/api/machines/{machine_a["id"]}/drawings?q=O9002'))['total'] == 0
        request(f'/api/machines/{machine_a["id"]}/enabled', {'enabled': False}, method='PUT')
        request(f'/api/operations/{operation["id"]}/programs', {'programNo': 'DISABLED', 'machineId': machine_a['id']}, 400)
        current = json.loads(request(f'/api/programs/{program["id"]}', {
            'programNo': 'O9001', 'expectedRevision': current['revision'], 'machineId': machine_a['id']}, method='PUT'))
        versions_before = json.loads(request(f'/api/programs/{program["id"]}/versions'))
        tools_before = json.loads(request(f'/api/versions/{version["id"]}/tools'))
        request(f'/api/machines/{machine_a["id"]}', expected=204, method='DELETE')
        after_delete = json.loads(request(f'/api/programs/{program["id"]}'))
        assert after_delete['machineId'] is None and after_delete['revision'] == current['revision'] + 1
        assert json.loads(request(f'/api/programs/{sibling["id"]}'))['machineId'] == machine_b['id']
        assert json.loads(request(f'/api/programs/{program["id"]}/versions')) == versions_before
        assert json.loads(request(f'/api/versions/{version["id"]}/tools')) == tools_before
        assert request(f'/api/versions/{version["id"]}/download') == content
        request(f'/api/programs/{program["id"]}', {
            'programNo': 'STALE', 'expectedRevision': current['revision'], 'machineId': machine_a['id']}, 409, method='PUT')
        new_machine = json.loads(request('/api/machines', {'code': 'CNC-NEW'}, 201))
        assert new_machine['id'] > machine_b['id']
        request(f'/api/operations/{operation["id"]}/programs', {'programNo': 'DELETED', 'machineId': machine_a['id']}, 400)
        checks.extend(['machine-create', 'independent-program-machines', 'machine-related-program-search',
                       'disabled-machine-preserves-existing', 'machine-delete-preserves-history',
                       'deleted-machine-stale-form-conflict', 'deleted-machine-id-not-reused'])
        backup = json.loads(request("/api/backup/create", {}, 201))
        archive = request("/api/backup/recoveries/" + backup["name"])
        with zipfile.ZipFile(io.BytesIO(archive)) as bundle:
            assert bundle.testzip() is None
            assert any(entry.endswith("cnc.db") for entry in bundle.namelist())
        checks.extend(["create-business-records", "invalid-drawing-400", "nc-upload-download", "backup-create-download"])
        key_hash = docker("exec", name, "sha256sum", "/data/nc.key").split()[0]
        for _ in range(100):
            health = docker("inspect", "--format", "{{.State.Health.Status}}", name)
            if health == "healthy":
                break
            time.sleep(0.5)
        assert health == "healthy", health
        checks.append("healthcheck")
        docker("stop", "-t", "15", name)
        assert docker("inspect", "--format", "{{.State.ExitCode}}", name) == "0"
        docker("rm", name)
        start()
        assert not json.loads(request("/api/auth/status"))["setupRequired"]
        assert json.loads(request("/api/auth/me"))["id"] == user["id"]
        assert json.loads(request(f'/api/drawings/{drawing["id"]}/detail'))["drawingNo"] == "DOCKER-SMOKE"
        assert request(f'/api/versions/{version["id"]}/download') == content
        assert docker("exec", name, "sha256sum", "/data/nc.key").split()[0] == key_hash
        assert request("/api/backup/recoveries/" + backup["name"]) == archive
        assert json.loads(request(f'/api/programs/{sibling["id"]}'))['machineId'] == machine_b['id']
        checks.extend(["graceful-stop", "container-recreation", "account-session-persistence",
                       "business-data-persistence", "nc-key-persistence", "backup-persistence"])
        invalid = subprocess.run(["docker", "run", "--rm", "-e", "CNC_MAX_UPLOAD_MB=0", args.image],
                                 capture_output=True, text=True, timeout=20)
        assert invalid.returncode != 0 and "CNC_MAX_UPLOAD_MB" in invalid.stdout + invalid.stderr
        checks.append("invalid-config-rejected")
        result.update({"passed": True, "imageId": docker("image", "inspect", "--format", "{{.Id}}", args.image),
                       "ncSha256": hashlib.sha256(content).hexdigest(), "keySha256": key_hash})
    except Exception as error:
        result.update({"passed": False, "error": str(error)})
        raise
    finally:
        logs = subprocess.run(["docker", "logs", name], capture_output=True, text=True)
        report.with_suffix(".log").write_text(logs.stdout + logs.stderr)
        subprocess.run(["docker", "rm", "-f", name], capture_output=True)
        subprocess.run(["docker", "volume", "rm", volume], capture_output=True)
        report.write_text(json.dumps(result, indent=2, ensure_ascii=False) + "\n")
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
