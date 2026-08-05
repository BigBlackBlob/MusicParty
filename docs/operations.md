# MusicParty 运维手册

**版本**: v2.0  
**更新日期**: 2026-08-04  
**适用环境**: Docker / 独立部署

---

## 📋 目录

1. [部署指南](#部署指南)
2. [配置说明](#配置说明)
3. [监控与日志](#监控与日志)
4. [备份与恢复](#备份与恢复)
5. [故障排查](#故障排查)
6. [性能优化](#性能优化)
7. [安全加固](#安全加固)
8. [常见问题](#常见问题)

---

## 🚀 部署指南

### 快速部署 (Docker Compose)

#### 1. 准备环境
```bash
# 安装 Docker 和 Docker Compose
sudo apt update
sudo apt install docker.io docker-compose

# 克隆仓库
git clone https://github.com/BigBlackBlob/MusicParty.git
cd MusicParty
```

#### 2. 配置环境变量
```bash
# 复制配置模板
cp .env.example .env

# 编辑配置（必填项）
nano .env
```

**必填配置**:
```bash
# 网易云 API 镜像
NETEASE_API_IMAGE=binaryify/neteasecloudmusicapi:4.21.3

# MusicParty 镜像（使用 digest）
MUSIC_PARTY_IMAGE=ghcr.io/bigblackblob/musicparty@sha256:your-digest

# 公开访问 URL
BASE_URL=https://music.example.com
ALLOWED_ORIGINS=https://music.example.com

# 管理员账号（首次启动时创建）
BOOTSTRAP_ADMIN_USERNAME=admin
BOOTSTRAP_ADMIN_PASSWORD=YourSecurePassword123
```

#### 3. 启动服务
```bash
# 启动所有服务
docker compose up -d

# 查看日志
docker compose logs -f music-party

# 验证服务
curl http://localhost:8848/actuator/health
```

#### 4. 访问应用
打开浏览器访问: `http://localhost:8848`

---

### 生产环境部署

#### 推荐配置
- **CPU**: 2 核心
- **内存**: 2GB
- **磁盘**: 20GB (包含音频缓存)
- **网络**: 10 Mbps

#### 使用 Nginx 反向代理
```nginx
server {
    listen 80;
    server_name music.example.com;

    # 强制 HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name music.example.com;

    ssl_certificate /etc/ssl/certs/music.example.com.crt;
    ssl_certificate_key /etc/ssl/private/music.example.com.key;

    # WebSocket 支持
    location /ws {
        proxy_pass http://localhost:8848;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket 超时
        proxy_read_timeout 86400;
        proxy_send_timeout 86400;
    }

    # 静态文件缓存
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        proxy_pass http://localhost:8848;
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # API 和其他请求
    location / {
        proxy_pass http://localhost:8848;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # 上传限制
        client_max_body_size 220M;
    }
}
```

---

## ⚙️ 配置说明

### 配置模板

#### 小型部署 (1-10 用户)
```bash
# 基础配置
BASE_URL=http://localhost:8848

# 性能配置（保守）
SQLITE_BUSY_TIMEOUT=5000
SQLITE_READ_CONNECTIONS=2
SQLITE_WRITE_QUEUE_CAPACITY=100

# 媒体缓存（小）
MEDIA_CACHE_SIZE_MB=500
MEDIA_DOWNLOAD_MAX_QUEUED=50
MEDIA_DOWNLOAD_CONCURRENCY_NETEASE=2
MEDIA_DOWNLOAD_CONCURRENCY_BILIBILI=1

# 限流（宽松）
QUEUE_MAX_USER_SONGS=50
CHAT_MIN_INTERVAL=1000
CHAT_MAX_LENGTH=200
```

#### 中型部署 (10-50 用户)
```bash
# 性能配置（标准）
SQLITE_BUSY_TIMEOUT=10000
SQLITE_READ_CONNECTIONS=4
SQLITE_WRITE_QUEUE_CAPACITY=200

# 媒体缓存（中）
MEDIA_CACHE_SIZE_MB=2000
MEDIA_DOWNLOAD_MAX_QUEUED=100
MEDIA_DOWNLOAD_CONCURRENCY_NETEASE=3
MEDIA_DOWNLOAD_CONCURRENCY_BILIBILI=2

# 限流（标准）
QUEUE_MAX_USER_SONGS=100
CHAT_MIN_INTERVAL=500
CHAT_MAX_LENGTH=500
```

#### 大型部署 (50-200 用户)
```bash
# 性能配置（激进）
SQLITE_BUSY_TIMEOUT=15000
SQLITE_READ_CONNECTIONS=8
SQLITE_WRITE_QUEUE_CAPACITY=500

# 媒体缓存（大）
MEDIA_CACHE_SIZE_MB=5000
MEDIA_DOWNLOAD_MAX_QUEUED=200
MEDIA_DOWNLOAD_CONCURRENCY_NETEASE=5
MEDIA_DOWNLOAD_CONCURRENCY_BILIBILI=3

# 限流（严格）
QUEUE_MAX_USER_SONGS=200
CHAT_MIN_INTERVAL=300
CHAT_MAX_LENGTH=1000
```

### 环境变量完整列表

#### 必填配置
| 变量 | 说明 | 示例 |
|------|------|------|
| `BASE_URL` | 公开访问 URL | `https://music.example.com` |
| `ALLOWED_ORIGINS` | 浏览器与 WebSocket 允许来源 | `https://music.example.com` |
| `NETEASE_API_URL` | 网易云 API 地址 | `http://netease-api:3000` |
| `BOOTSTRAP_ADMIN_USERNAME` | 管理员用户名 | `admin` |
| `BOOTSTRAP_ADMIN_PASSWORD` | 管理员密码 | `SecurePass123` |

#### 可选配置
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `LOG_LEVEL` | `INFO` | 日志级别 (DEBUG/INFO/WARN/ERROR) |
| `SQLITE_BUSY_TIMEOUT` | `5000` | SQLite 忙等待超时 (ms) |
| `SQLITE_READ_CONNECTIONS` | `2` | SQLite 读连接池大小 |
| `MEDIA_CACHE_SIZE_MB` | `1024` | 媒体缓存上限 (MB) |
| `QUEUE_MAX_USER_SONGS` | `100` | 单用户最大点歌数 |
| `CHAT_MIN_INTERVAL` | `1000` | 聊天最小间隔 (ms) |

---

## 📊 监控与日志

### Prometheus 指标

**暴露端点**:
```
GET /actuator/prometheus
```

**推荐监控指标**:
```promql
# HTTP 请求率
rate(http_requests_total[5m])

# HTTP 错误率
rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m])

# P95 延迟
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# WebSocket 连接数
websocket_connections

# 房间活跃用户数
sum(room_active_users) by (room_id)
```

**Prometheus 配置示例**:
```yaml
scrape_configs:
  - job_name: 'musicparty'
    static_configs:
      - targets: ['localhost:8848']
    metrics_path: '/actuator/prometheus'
    scrape_interval: 15s
```

### 日志管理

**日志格式**: 结构化 JSON
```json
{
  "time": "2026-08-04T10:30:45Z",
  "level": "INFO",
  "msg": "HTTP request",
  "requestId": "abc123",
  "method": "GET",
  "path": "/api/rooms",
  "status": 200,
  "durationMs": 15,
  "clientIp": "192.168.1.100"
}
```

**查看实时日志**:
```bash
# 查看所有日志
docker compose logs -f music-party

# 过滤错误日志
docker compose logs music-party | grep '"level":"ERROR"'

# 查看最近 100 条
docker compose logs --tail=100 music-party
```

**日志聚合 (ELK Stack)**:
```bash
# Filebeat 配置
filebeat.inputs:
  - type: docker
    containers.ids: '*'
    json.keys_under_root: true
    json.add_error_key: true

output.elasticsearch:
  hosts: ["elasticsearch:9200"]
```

### 健康检查

**端点**:
```bash
# 综合健康状态
curl http://localhost:8848/actuator/health

# 存活探针（K8s liveness）
curl http://localhost:8848/actuator/health/liveness

# 就绪探针（K8s readiness）
curl http://localhost:8848/actuator/health/readiness
```

**K8s 配置示例**:
```yaml
livenessProbe:
  httpGet:
    path: /actuator/health/liveness
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /actuator/health/readiness
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 2
```

---

## 💾 备份与恢复

### 数据备份

**备份内容**:
- SQLite 数据库: `./music_party/data/musicparty.db`
- 本地曲库: `./music_party/data/local-library/`
- 音频缓存 (可选): `./music_party/cached_media/`

**自动备份脚本**:
```bash
#!/bin/bash
# backup.sh

BACKUP_DIR="/backup/musicparty"
DATA_DIR="./music_party/data"
DATE=$(date +%Y%m%d_%H%M%S)

# 创建备份目录
mkdir -p "$BACKUP_DIR"

# 备份数据库（使用 .backup 命令保证一致性）
sqlite3 "$DATA_DIR/musicparty.db" ".backup '$BACKUP_DIR/musicparty_$DATE.db'"

# 备份本地曲库
tar -czf "$BACKUP_DIR/local-library_$DATE.tar.gz" -C "$DATA_DIR" local-library

# 保留最近 7 天的备份
find "$BACKUP_DIR" -name "musicparty_*.db" -mtime +7 -delete
find "$BACKUP_DIR" -name "local-library_*.tar.gz" -mtime +7 -delete

echo "Backup completed: $DATE"
```

**定时备份 (Cron)**:
```bash
# 每天凌晨 2 点备份
0 2 * * * /path/to/backup.sh >> /var/log/musicparty-backup.log 2>&1
```

### 数据恢复

**从备份恢复**:
```bash
# 停止服务
docker compose down

# 恢复数据库
cp /backup/musicparty/musicparty_20260804_020000.db ./music_party/data/musicparty.db

# 恢复本地曲库
tar -xzf /backup/musicparty/local-library_20260804_020000.tar.gz -C ./music_party/data/

# 启动服务
docker compose up -d
```

**灾难恢复检查清单**:
- [ ] 验证数据库完整性: `sqlite3 musicparty.db "PRAGMA integrity_check;"`
- [ ] 检查文件权限: `chown -R 1000:1000 ./music_party/data`
- [ ] 验证服务启动: `docker compose logs -f music-party`
- [ ] 测试登录功能
- [ ] 测试播放功能

---

## 🔧 故障排查

### 常见问题诊断

#### 问题 1: 服务无法启动
**症状**: `docker compose up` 失败

**排查步骤**:
```bash
# 1. 查看日志
docker compose logs music-party

# 2. 检查端口占用
lsof -i :8848

# 3. 检查配置文件
cat .env | grep -v '^#'

# 4. 检查数据库文件权限
ls -la ./music_party/data/
```

**常见原因**:
- 端口被占用 → 修改 `docker-compose.yml` 端口映射
- 数据库文件权限错误 → `chown -R 1000:1000 ./music_party`
- 缺少环境变量 → 检查 `.env` 文件

#### 问题 2: WebSocket 连接失败
**症状**: 前端显示 "连接断开"

**排查步骤**:
```bash
# 1. 检查 WebSocket 端点
curl -i -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" \
  -H "Sec-WebSocket-Key: test" \
  http://localhost:8848/ws

# 2. 检查防火墙
sudo ufw status

# 3. 检查 Nginx 配置（如果使用）
nginx -t
```

**常见原因**:
- Nginx 未配置 WebSocket 升级
- 防火墙阻止连接
- 客户端网络问题

#### 问题 3: 数据库锁定
**症状**: 大量 "database is locked" 错误

**排查步骤**:
```bash
# 1. 检查 SQLite 进程
lsof ./music_party/data/musicparty.db

# 2. 增加 busy timeout
# 在 .env 中设置
SQLITE_BUSY_TIMEOUT=15000

# 3. 检查写队列容量
# 在日志中搜索 "write queue full"
docker compose logs music-party | grep "queue full"
```

**解决方案**:
- 增加 `SQLITE_BUSY_TIMEOUT`
- 增加 `SQLITE_WRITE_QUEUE_CAPACITY`
- 考虑迁移到 PostgreSQL（如果用户数 > 200）

#### 问题 4: 内存占用过高
**症状**: 容器 OOM 或性能下降

**排查步骤**:
```bash
# 1. 查看容器资源使用
docker stats music-party-app

# 2. 检查缓存大小
du -sh ./music_party/cached_media

# 3. 检查在线用户数
curl http://localhost:8848/actuator/health | jq '.websocket.connections'
```

**优化方案**:
- 减少 `MEDIA_CACHE_SIZE_MB`
- 减少 `SQLITE_READ_CONNECTIONS`
- 增加容器内存限制: `docker compose` 中设置 `mem_limit`

---

## ⚡ 性能优化

### SQLite 优化

**推荐配置**:
```bash
# 启用 WAL 模式（已默认启用）
sqlite3 musicparty.db "PRAGMA journal_mode=WAL;"

# 增加缓存大小
sqlite3 musicparty.db "PRAGMA cache_size=-64000;"  # 64MB

# 同步模式
sqlite3 musicparty.db "PRAGMA synchronous=NORMAL;"
```

### 网络优化

**启用 Gzip 压缩** (Nginx):
```nginx
gzip on;
gzip_types text/plain text/css application/json application/javascript text/xml application/xml;
gzip_min_length 1000;
```

**启用 HTTP/2**:
```nginx
listen 443 ssl http2;
```

### 缓存策略

**媒体文件缓存**:
```bash
# 增加缓存大小（适合大型部署）
MEDIA_CACHE_SIZE_MB=5000

# 增加下载并发
MEDIA_DOWNLOAD_CONCURRENCY_NETEASE=5
MEDIA_DOWNLOAD_CONCURRENCY_BILIBILI=3
```

---

## 🔒 安全加固

### 1. 密码策略
```bash
# 强制管理员使用强密码（至少 12 字符）
BOOTSTRAP_ADMIN_PASSWORD=YourVerySecurePassword123!
```

### 2. HTTPS 强制
```bash
# 设置公开 URL 为 HTTPS
MUSIC_PARTY_PUBLIC_URL=https://music.example.com

# 在 Nginx 中强制跳转
return 301 https://$server_name$request_uri;
```

### 3. 限制访客权限
```bash
# 严格限流
QUEUE_MAX_USER_SONGS=50  # 访客点歌限制
CHAT_MIN_INTERVAL=3000   # 访客聊天间隔更长
```

### 4. IP 白名单（管理员 API）
使用 Nginx 限制管理 API 访问:
```nginx
location /api/admin/ {
    allow 192.168.1.0/24;  # 内网
    deny all;
    proxy_pass http://localhost:8848;
}
```

### 5. 定期更新
```bash
# 检查 Go 依赖漏洞
cd backend-go
govulncheck ./...

# 检查 NPM 依赖漏洞
cd music-party-web
npm audit
npm audit fix
```

---

## ❓ 常见问题 (FAQ)

### Q1: 如何重置管理员密码？
```bash
# 停止服务
docker compose down

# 删除管理员账号
sqlite3 ./music_party/data/musicparty.db "DELETE FROM user_account WHERE role='PLATFORM_ADMIN';"

# 修改 .env 中的密码
nano .env

# 重启服务（自动重新创建管理员）
docker compose up -d
```

### Q2: 如何迁移到新服务器？
```bash
# 旧服务器：备份数据
tar -czf musicparty-backup.tar.gz ./music_party

# 新服务器：恢复数据
tar -xzf musicparty-backup.tar.gz
docker compose up -d
```

### Q3: 如何清理缓存？
```bash
# 清理媒体缓存
rm -rf ./music_party/cached_media/*

# 重启服务
docker compose restart music-party
```

### Q4: 如何查看当前版本？
```bash
# 查看容器镜像版本
docker inspect music-party-app | grep -i image

# 查看 Git commit
cd MusicParty
git log -1 --oneline
```

### Q5: 如何扩展到多实例？
**注意**: 当前架构不支持多实例水平扩展（SQLite 单文件限制）

**如需扩展**:
1. 迁移到 PostgreSQL
2. 使用 Redis 存储 WebSocket 会话
3. 配置负载均衡器（Nginx/HAProxy）

---

## 📞 支持与联系

- **GitHub Issues**: https://github.com/BigBlackBlob/MusicParty/issues
- **文档**: `docs/` 目录
- **架构图**: `docs/architecture.md`

---

**更新日志**:
- 2026-08-04: 初始版本，包含访客模式配置
- 持续更新中...
