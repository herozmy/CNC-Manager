#!/usr/bin/env bash
# Remote-only Docker builds. Personal SSH settings stay in ignored .local/.
set -euo pipefail
cd "$(dirname "$0")/.."
case "${1:-build}" in
  build|push) action="${1:-build}" ;;
  *) echo "用法：bash scripts/docker-remote.sh [build|push]" >&2; exit 2 ;;
esac
if [[ -f .local/docker-remote.env ]]; then
  source .local/docker-remote.env
fi
: "${CNC_DOCKER_HOST:?请配置 CNC_DOCKER_HOST（SSH 主机别名）}"
: "${CNC_DOCKER_WORK_DIR:?请配置 CNC_DOCKER_WORK_DIR（远端工作目录）}"
image="${CNC_DOCKER_IMAGE:-herozmy/cnc-manager}"
[[ "$CNC_DOCKER_HOST" =~ ^[a-zA-Z0-9_.@-]+$ && "$CNC_DOCKER_HOST" != -* ]] || exit 2
[[ "$CNC_DOCKER_WORK_DIR" =~ ^/[a-zA-Z0-9_./-]+$ ]] || exit 2
[[ "$image" =~ ^[a-z0-9][a-z0-9._/-]*$ ]] || exit 2
mkdir -p .local/docker
if [[ "$action" == build ]]; then
  revision="$(git rev-parse HEAD)"
  if [[ -n "$(git status --porcelain)" ]]; then
    revision="$revision-dirty"
  fi
  snapshot="$(mktemp -d .local/docker/source.XXXXXX)"
  trap 'rm -rf "$snapshot"' EXIT
  # Export only Git-visible application files, never local data or credentials.
  python3 - "$snapshot" <<'PY'
import hashlib, pathlib, shutil, subprocess, sys
destination = pathlib.Path(sys.argv[1])
files = subprocess.check_output(["git", "ls-files", "-co", "--exclude-standard", "-z"]).split(b"\0")
manifest = []
for raw in sorted(set(files)):
    if not raw:
        continue
    path = pathlib.Path(raw.decode())
    if path.parts[0] not in {"frontend", "backend", "scripts", "Dockerfile", ".dockerignore", "VERSION", "compose.yaml", "compose.build.yaml"}:
        continue
    if any(part in {"node_modules", "data", "dist", ".git", ".local", "__pycache__"} or part.startswith(".env") for part in path.parts):
        continue
    if path.suffix.lower() in {".md", ".markdown", ".log"} or path.as_posix() == "backend/server":
        continue
    if path.is_symlink() or not path.is_file():
        continue
    target = destination / path
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(path, target)
    manifest.append(f"{hashlib.sha256(target.read_bytes()).hexdigest()}  {path.as_posix()}\n")
(destination / "SOURCE_SHA256SUMS").write_text("".join(manifest))
PY
  build_dir="${CNC_DOCKER_WORK_DIR%/}/cnc-manager-v2/builds/$(date +%Y%m%d-%H%M%S)-${revision:0:12}-$(basename "$snapshot")"
  printf -v remote_command '%q ' mkdir -p -- "$build_dir"
  ssh -o BatchMode=yes "$CNC_DOCKER_HOST" "$remote_command"
  rsync -a -e 'ssh -o BatchMode=yes' "$snapshot/" "$CNC_DOCKER_HOST:$build_dir/"
else
  [[ -f .local/docker/last-build ]] || { echo "请先完成远端 build" >&2; exit 1; }
  build_dir="$(cat .local/docker/last-build)"
fi
printf -v remote_command '%q ' bash -s -- "$action" "$build_dir" "$CNC_DOCKER_WORK_DIR" "$image" "${revision:-unknown}"
ssh -o BatchMode=yes "$CNC_DOCKER_HOST" "$remote_command" <<'REMOTE'
set -euo pipefail
action="$1"; build_dir="$2"; work_dir="$3"; image="$4"; revision="$5"
work_dir="$(cd "$work_dir" && pwd -P)"
cd "$build_dir"
[[ "$(pwd -P)/" == "$work_dir/"* ]] || { echo "构建/推送必须在指定远端工作目录下执行" >&2; exit 1; }
sha256sum -c SOURCE_SHA256SUMS > /dev/null
version="$(tr -d '[:space:]' < VERSION)"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || exit 2
version_tag="$image:v$version"
rolling_tag="$image:v2"
latest_tag="$image:latest"
mkdir -p .local
if [[ "$action" == build ]]; then
  docker build --platform linux/amd64 --build-arg "VCS_REF=$revision" \
    --tag "$version_tag" --tag "$rolling_tag" .
  [[ "$(docker run --rm "$version_tag" --version)" == "$version" ]]
  python3 scripts/docker-smoke.py --image "$version_tag" --version "$version" --report .local/docker-smoke.json
  python3 scripts/migration-smoke.py --image "$version_tag" --version "$version" --report .local/migration-smoke.json
  python3 scripts/web-migration-smoke.py --image "$version_tag" --version "$version" --report .local/web-migration-smoke.json
  docker image inspect --format '{{.Id}}' "$version_tag" > .local/image-id
  printf '%s\n' "$image" > .local/image-repository
  echo "远端镜像已构建并核对版本：$version_tag（未推送）"
else
  [[ "$(cat .local/image-repository)" == "$image" ]]
  [[ "$(docker image inspect --format '{{.Id}}' "$version_tag")" == "$(cat .local/image-id)" ]]
  # Reuse an identical published image after a partial push; never overwrite a release tag.
  push_version=true
  if docker manifest inspect "$version_tag" > .local/remote-manifest.json 2> .local/manifest-error; then
    python3 - .local/remote-manifest.json "$(cat .local/image-id)" <<'PY'
import json, sys
manifest = json.load(open(sys.argv[1]))
if manifest.get("config", {}).get("digest") != sys.argv[2]:
    sys.exit("版本标签已存在且不是本次已验证的镜像，停止推送")
PY
    push_version=false
  else
    manifest_error="$(cat .local/manifest-error)"
    if [[ "$manifest_error" != *"manifest unknown"* && "$manifest_error" != *"no such manifest"* ]]; then
      cat .local/manifest-error >&2; exit 1
    fi
  fi
  docker tag "$(cat .local/image-id)" "$rolling_tag"
  docker tag "$(cat .local/image-id)" "$latest_tag"
  if [[ "$push_version" == true ]]; then
    docker push "$version_tag"
  fi
  docker push "$rolling_tag"
  docker push "$latest_tag"
  version_digest="$(docker buildx imagetools inspect "$version_tag" | awk '$1 == "Digest:" {print $2; exit}')"
  rolling_digest="$(docker buildx imagetools inspect "$rolling_tag" | awk '$1 == "Digest:" {print $2; exit}')"
  latest_digest="$(docker buildx imagetools inspect "$latest_tag" | awk '$1 == "Digest:" {print $2; exit}')"
  [[ "$version_digest" == sha256:* && "$version_digest" == "$rolling_digest" && "$version_digest" == "$latest_digest" ]]
  printf '%s %s\n%s %s\n%s %s\n' "$version_tag" "$version_digest" "$rolling_tag" "$rolling_digest" "$latest_tag" "$latest_digest" | tee .local/pushed-digests
  docker pull --platform linux/amd64 "$latest_tag"
  [[ "$(docker image inspect --format '{{.Id}}' "$latest_tag")" == "$(cat .local/image-id)" ]]
  [[ "$(docker run --rm "$latest_tag" --version)" == "$version" ]]
  python3 scripts/docker-smoke.py --image "$latest_tag" --version "$version" --report .local/docker-published-smoke.json
  python3 scripts/migration-smoke.py --image "$latest_tag" --version "$version" --report .local/migration-published-smoke.json
  python3 scripts/web-migration-smoke.py --image "$latest_tag" --version "$version" --report .local/web-migration-published-smoke.json
fi
REMOTE
if [[ "$action" == build ]]; then
  printf '%s\n' "$build_dir" > .local/docker/last-build
fi
