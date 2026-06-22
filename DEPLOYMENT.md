# Sub2API 二开分支 GHCR 镜像部署

本仓库的二开分支是 `sub2-fork`。推送到该分支后，GitHub Actions 会自动构建 Docker 镜像并推送到 GHCR。

## 自动构建

Workflow：`.github/workflows/docker-build.yml`

触发方式：

- push 到 `sub2-fork`
- 在 GitHub Actions 页面手动运行 `Build Sub2 Fork Docker Image`

镜像标签：

- `ghcr.io/yunqiasen/sub2api:sub2-fork`
- `ghcr.io/yunqiasen/sub2api:<VERSION>-sub2-fork`
- `ghcr.io/yunqiasen/sub2api:sha-<short_sha>`

日常服务器部署建议用：

```yaml
image: ghcr.io/yunqiasen/sub2api:sub2-fork
```

需要固定版本或回滚时，用版本标签或 sha 标签。

## 服务器部署原则

服务器不需要 clone 仓库，也不要热挂载整个源码目录。

只挂载运行数据：

- `.env` 或服务环境变量
- PostgreSQL 数据目录
- Redis 数据目录
- 应用数据目录
- 日志目录

源码热挂载只用于本地开发。

## docker-compose 示例

按服务器实际端口、密码和路径调整。

```yaml
services:
  sub2api:
    image: ghcr.io/yunqiasen/sub2api:sub2-fork
    container_name: sub2api
    restart: unless-stopped
    env_file:
      - .env
    ports:
      - "8420:8080"
    volumes:
      - ./data:/app/data
      - ./logs:/app/logs
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    labels:
      com.centurylinklabs.watchtower.enable: "true"

  postgres:
    image: postgres:18-alpine
    container_name: sub2api-postgres
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-sub2api}
      POSTGRES_USER: ${POSTGRES_USER:-sub2api}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}
    volumes:
      - ./postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-sub2api} -d ${POSTGRES_DB:-sub2api}"]
      interval: 10s
      timeout: 5s
      retries: 10

  redis:
    image: redis:8-alpine
    container_name: sub2api-redis
    restart: unless-stopped
    command: redis-server --appendonly yes
    volumes:
      - ./redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 10

  watchtower:
    image: containrrr/watchtower:latest
    container_name: watchtower
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    command: --label-enable --cleanup --interval 300
```

## 部署命令

首次部署：

```bash
docker compose pull
docker compose up -d
```

手动更新：

```bash
docker compose pull sub2api
docker compose up -d sub2api
```

查看状态：

```bash
docker compose ps
docker compose logs --tail=120 sub2api
```

## GHCR 权限

如果 GHCR package 是私有的，服务器先登录：

```bash
echo '<PAT>' | docker login ghcr.io -u '<github_user>' --password-stdin
```

如果要免登录拉取，把 GHCR package visibility 调成 Public。

## Watchtower 自动更新

示例只允许 Watchtower 更新带这个 label 的容器：

```yaml
labels:
  com.centurylinklabs.watchtower.enable: "true"
```

Watchtower 参数：

```bash
--label-enable --cleanup --interval 300
```

这样只会自动更新明确标记的服务，不会误动数据库、Redis 或其他容器。
