# Docker 部署手册

本文适用于在 Linux x86-64 服务器上使用 Docker Compose 部署 CNC-Manager。应用采用单容器架构：Go 服务同时提供 API 和 Vue 页面，SQLite 数据库、NC 文件、备份和日志均保存在宿主机目录中。

## 1. 准备服务器

建议使用 Ubuntu 22.04/24.04、Debian 12 或其他支持 Docker Engine 与 Compose V2 的发行版。确认命令可用：

```bash
docker --version
docker compose version
```

防火墙只需允许实际使用的端口。仅限局域网访问时，可以开放 TCP 8080；公网部署应仅开放 80/443，并通过反向代理提供 HTTPS。

## 2. 获取源码

以下示例把项目部署到 `/opt/cnc-manager-deploy`：

```bash
sudo mkdir -p /opt/cnc-manager-deploy
sudo chown "$USER":"$USER" /opt/cnc-manager-deploy
git clone https://github.com/herozmy/CNC-Manager.git /opt/cnc-manager-deploy
cd /opt/cnc-manager-deploy
```

如果服务器不能访问 GitHub，也可以从本地把整个项目目录上传到该位置。

## 3. 配置运行用户和持久化目录

镜像支持通过 `PUID` 和 `PGID` 指定运行身份。默认值都是 `0`，即以 root 运行，适合直接挂载 NAS 目录：

```yaml
environment:
  PUID: "0"
  PGID: "0"
```

需要以普通用户运行时，可以改成实际用户的 UID/GID，例如：

```yaml
environment:
  PUID: "1000"
  PGID: "1000"
```

可以在服务器上使用 `id 用户名` 查询。非 root 模式下，容器启动时会自动把 `/data` 调整为该 UID/GID，无需提前手工 `chown`。`PUID` 和 `PGID` 必须同时为 0，或者同时为非零整数。首次启动前只需创建目录：

```bash
cd /opt/cnc-manager-deploy
mkdir -p docker-data
```

该目录包含所有需要保留的数据：

```text
docker-data/
├── cnccool.db
├── cnccool.db-wal
├── cnccool.db-shm
├── nc/
├── backups/
└── logs/
```

不能只持久化 `nc/`，也不要让两个容器同时使用同一个 `docker-data/`。

## 4. 构建并启动

仓库中的 `compose.yaml` 已包含端口、数据卷和自动重启策略：

```bash
docker compose pull
docker compose up -d
docker compose ps
docker compose logs --tail=100 cnc-manager
```

浏览器访问：

```text
http://服务器IP:8080
```

首次打开时创建初始管理员。服务容器由 Docker 管理，不需要也不应在容器内运行 systemd；宿主机重启后，`restart: unless-stopped` 会自动恢复服务。

## 5. 日常管理

```bash
# 查看状态
docker compose ps

# 持续查看日志
docker compose logs -f cnc-manager

# 重启
docker compose restart cnc-manager

# 停止并删除容器（不会删除绑定目录中的数据）
docker compose down

# 再次启动
docker compose up -d
```

应用日志也会写入 `docker-data/logs/server.log`。

## 6. 更新版本

更新前先登录系统，在“备份管理”中创建并下载完整备份。随后在部署目录执行：

```bash
cd /opt/cnc-manager-deploy
docker compose pull
docker compose up -d
docker compose ps
docker compose logs --tail=100 cnc-manager
```

`docker compose up -d` 会用新镜像替换容器，但继续挂载原来的 `docker-data/`。启动时数据库迁移会自动执行。不要在 Docker 环境使用 Windows 版的界面内离线升级功能。

默认镜像为 `herozmy/cnc-manager:latest`。生产环境也可以把 `compose.yaml` 中的标签固定为发布版本，例如 `herozmy/cnc-manager:v1.2.10`，避免 `latest` 随后续发布变化。

Docker 版会检查 GitHub Release 并在界面提示新版本，但不会在容器内替换自身。更新弹窗会提示运行 `docker compose pull` 和 `docker compose up -d`；实际更新仍由部署管理员执行。

如果需要从当前源码构建而不是从 Docker Hub 拉取，执行：

```bash
docker compose -f compose.yaml -f compose.build.yaml up -d --build
```

如需回滚程序版本，应先恢复与旧程序兼容的完整数据备份，再切换到对应 Git tag 重建镜像；仅回滚镜像但保留已经迁移的新数据库可能不兼容。

## 7. 备份与恢复

推荐在界面的“备份管理”中生成完整迁移包，它包含 SQLite 一致性快照和全部 NC 文件。还可以在停止服务后备份整个持久化目录：

```bash
docker compose down
sudo tar -czf "cnc-manager-data-$(date +%Y%m%d-%H%M%S).tar.gz" docker-data
docker compose up -d
```

恢复整个目录前必须停止容器：

```bash
docker compose down
sudo mv docker-data "docker-data.before-restore-$(date +%Y%m%d-%H%M%S)"
sudo tar -xzf cnc-manager-data-YYYYMMDD-HHMMSS.tar.gz
# 如需改用另一组身份，在 compose.yaml 中设置 PUID/PGID 后启动即可
docker compose up -d
docker compose logs --tail=100 cnc-manager
```

不要在服务运行时直接复制 `cnccool.db`，因为 SQLite 使用 WAL 模式，单独复制数据库文件可能得到不一致的备份。当前界面中的“恢复并自动重启”依赖 Windows 批处理，不适用于 Linux 容器。

## 8. 修改端口和监听范围

默认映射所有网卡的 8080 端口：

```yaml
ports:
  - "8080:8080"
```

只允许服务器本机的反向代理访问时，改成：

```yaml
ports:
  - "127.0.0.1:8080:8080"
```

修改后执行：

```bash
docker compose up -d
```

## 9. HTTPS 反向代理

公网部署推荐让 Caddy、Nginx 或 Traefik 监听 443，并代理到 `127.0.0.1:8080`。以 Nginx 为例：

```nginx
server {
    listen 443 ssl http2;
    server_name cnc.example.com;

    client_max_body_size 64m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 300s;
    }
}
```

同时把 Compose 端口绑定改为 `127.0.0.1:8080:8080`，避免绕过 HTTPS 直接访问后端。证书配置按服务器现有方案完成。

## 10. 常见故障

### 容器反复重启或数据库无法写入

检查日志和容器设置的运行身份：

```bash
docker compose logs --tail=200 cnc-manager
ls -ld docker-data
docker compose config | grep -A3 environment
docker compose restart cnc-manager
```

### 8080 端口被占用

```bash
sudo ss -lntp | grep ':8080'
```

可停止占用程序，或把 Compose 左侧的宿主机端口改为其他端口，例如 `8081:8080`。

### 页面无法打开

```bash
docker compose ps
docker compose logs --tail=100 cnc-manager
curl -i http://127.0.0.1:8080/api/meta
```

若服务器本机可以访问而其他电脑不行，检查云安全组、防火墙和端口绑定。

### 磁盘空间不足

```bash
df -h
du -sh docker-data
docker system df
```

清理未使用的 Docker 构建缓存前，应确认没有依赖相关镜像；不要删除 `docker-data/`。

## 11. SSH 远程部署

建议先在本机配置密钥登录和 SSH 主机别名：

```sshconfig
Host cnc-server
    HostName 192.0.2.10
    User deploy
    IdentityFile ~/.ssh/id_ed25519
```

验证：

```bash
ssh cnc-server
```

远程协助时只需提供主机别名、部署目录和是否已有反向代理。不要通过聊天发送密码、私钥或令牌。
