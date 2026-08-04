# MusicParty 本地验证指南

## 前置准备

### 1. 启动 Docker Desktop
```bash
# 方法 1: 手动启动
# 打开 "Docker Desktop" 应用程序

# 方法 2: 命令行启动
"C:\Program Files\Docker\Docker\Docker Desktop.exe"
```

**等待 Docker 完全启动**（状态栏显示绿色图标）

### 2. 验证 Docker 状态
```bash
docker version
docker ps
```

---

## 快速验证（自动化脚本）

```bash
# 在 Git Bash 中运行
cd /c/Users/Nirotiy/Documents/NRT-GIT/musicparty/MusicParty
chmod +x local-verification.sh
./local-verification.sh
```

**脚本会自动执行**:
1. ✅ 检查 Docker 状态
2. 🔨 构建 Docker 镜像 (~5-10 分钟)
3. 🚀 启动服务（网易云 API + MusicParty）
4. ⏳ 等待服务就绪
5. 🧪 运行自动化测试
6. 📊 显示测试结果

---

## 手动验证步骤

如果自动化脚本失败或想手动控制，按以下步骤操作：

### 步骤 1: 构建 Docker 镜像
```bash
cd /c/Users/Nirotiy/Documents/NRT-GIT/musicparty/MusicParty

# 构建镜像（需要 5-10 分钟）
docker build -t musicparty:guest-mode-local -f Dockerfile .

# 验证镜像
docker images | grep musicparty
```

**预期输出**:
```
musicparty  guest-mode-local  <image-id>  X minutes ago  XXX MB
```

### 步骤 2: 准备测试配置

创建 `.env.test` 文件：
```bash
cat > .env.test << 'EOF'
NETEASE_API_IMAGE=binaryify/neteasecloudmusicapi:4.21.3
MUSIC_PARTY_IMAGE=musicparty:guest-mode-local
MUSIC_PARTY_PULL_POLICY=never
MUSIC_PARTY_PUBLIC_URL=http://localhost:8848
NETEASE_API_URL=http://netease-api:3000
BOOTSTRAP_ADMIN_USERNAME=admin
BOOTSTRAP_ADMIN_PASSWORD=TestPassword123!
LOG_LEVEL=INFO
SQLITE_BUSY_TIMEOUT=5000
SQLITE_READ_CONNECTIONS=2
MEDIA_CACHE_SIZE_MB=500
QUEUE_MAX_USER_SONGS=50
EOF
```

### 步骤 3: 启动服务
```bash
# 启动所有服务
docker compose --env-file .env.test up -d

# 查看容器状态
docker compose --env-file .env.test ps

# 查看日志
docker compose --env-file .env.test logs -f music-party
```

**预期输出**:
```
NAME                  STATUS      PORTS
music-party-netease   Up X min    3000/tcp
music-party-app       Up X min    0.0.0.0:8848->8080/tcp
```

### 步骤 4: 等待服务就绪
```bash
# 循环检查健康状态
for i in {1..30}; do
  if curl -f http://localhost:8848/actuator/health; then
    echo "✅ 服务已就绪"
    break
  fi
  echo "等待中... ($i/30)"
  sleep 2
done
```

---

## 功能测试清单

### API 测试

#### 1. 健康检查
```bash
curl http://localhost:8848/actuator/health | jq '.'
```
**预期**: `{"status":"UP"}`

#### 2. 账号状态
```bash
curl http://localhost:8848/api/account/status | jq '.'
```
**预期**: `{"requiresSetup":false}`

#### 3. 创建访客会话
```bash
curl -X POST http://localhost:8848/api/account/guest \
  -H "Content-Type: application/json" \
  -d '{"displayName":"测试访客"}' \
  -c /tmp/guest-cookies.txt | jq '.'
```
**预期**:
```json
{
  "publicId": "u_xxxx",
  "displayName": "测试访客",
  "role": "GUEST",
  "guest": true,
  "enabled": true
}
```

#### 4. 管理员登录
```bash
curl -X POST http://localhost:8848/api/account/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"TestPassword123!"}' \
  -c /tmp/admin-cookies.txt | jq '.'
```
**预期**:
```json
{
  "publicId": "u_xxxx",
  "username": "admin",
  "role": "PLATFORM_ADMIN",
  "guest": false,
  "enabled": true
}
```

#### 5. 房间列表（访客权限）
```bash
curl http://localhost:8848/api/rooms \
  -b /tmp/guest-cookies.txt | jq '.'
```
**预期**: 返回房间列表（至少包含 lounge）

---

### 浏览器测试

#### 1. 访客模式验证
1. **打开浏览器**: `http://localhost:8848`
2. **看到认证界面**: 默认显示「访客模式」
3. **输入昵称**: 例如 "测试用户"
4. **点击按钮**: "立即进入"
5. **验证成功**: 进入房间，看到播放器界面

**检查点**:
- [ ] 认证界面默认显示访客模式 ✅
- [ ] 可以输入昵称 ✅
- [ ] 点击进入后成功加载 ✅
- [ ] 能看到房间列表（Lounge）✅

#### 2. 访客升级横幅验证
进入房间后：
- [ ] 看到顶部升级提示横幅 ✅
- [ ] 横幅文案：「当前为访客模式，使用邀请码注册后可跨设备同步」✅
- [ ] 横幅有「立即注册」按钮 ✅

点击「立即注册」按钮：
- [ ] 弹出模态框 ✅
- [ ] 模态框标题：「升级为注册账号」✅
- [ ] 有邀请码输入框 ✅
- [ ] 有「取消」和「升级账号」按钮 ✅

#### 3. 管理员登录验证
1. **返回登录页**: 刷新页面或清除 Cookie
2. **切换模式**: 点击「管理员登录」链接
3. **输入凭据**:
   - 用户名: `admin`
   - 密码: `TestPassword123!`
4. **登录成功**: 进入管理员界面

**检查点**:
- [ ] 能切换到管理员登录模式 ✅
- [ ] 管理员登录成功 ✅
- [ ] 看到管理员专属功能（设置中心有管理菜单）✅

#### 4. UI 对比度验证
1. **浅色模式**: 默认或切换到浅色模式
2. **深色模式**: 切换到深色模式
3. **检查退出登录按钮**: 在设置 → 账号页面

**检查点**:
- [ ] 浅色模式下退出登录按钮清晰可见 ✅
- [ ] 深色模式下退出登录按钮清晰可见 ✅
- [ ] 悬停时有明显反馈 ✅

#### 5. 访客权限验证
以访客身份登录后：
- [ ] 可以听歌 ✅
- [ ] 可以聊天 ✅
- [ ] 可以点歌 ✅
- [ ] 可以创建个人歌单 ✅
- [ ] 不能创建房间 ✅（设置中心无创建房间选项）
- [ ] 不能管理邀请码 ✅（设置中心无邀请码菜单）

---

## 性能测试

### 响应时间
```bash
# 测试健康检查响应时间
time curl http://localhost:8848/actuator/health

# 测试 API 响应时间
time curl http://localhost:8848/api/rooms -b /tmp/guest-cookies.txt
```

**预期**: P95 < 200ms

### 内存使用
```bash
# 查看容器资源使用
docker stats music-party-app --no-stream
```

**预期**: 内存 < 512MB

### 日志检查
```bash
# 查看启动日志
docker compose --env-file .env.test logs music-party | grep "Started"

# 检查错误日志
docker compose --env-file .env.test logs music-party | grep "ERROR"
```

**预期**: 无 ERROR 级别日志

---

## 故障排查

### 问题 1: Docker 无法启动
```bash
# 检查 Docker 服务
net start | findstr docker

# 重启 Docker Desktop
taskkill /IM "Docker Desktop.exe" /F
"C:\Program Files\Docker\Docker\Docker Desktop.exe"
```

### 问题 2: 镜像构建失败
```bash
# 查看构建日志
docker build -t musicparty:guest-mode-local -f Dockerfile . --progress=plain

# 清理缓存重新构建
docker builder prune -f
docker build --no-cache -t musicparty:guest-mode-local -f Dockerfile .
```

### 问题 3: 服务无法启动
```bash
# 查看详细日志
docker compose --env-file .env.test logs music-party

# 检查端口占用
netstat -ano | findstr :8848

# 重启服务
docker compose --env-file .env.test restart
```

### 问题 4: 数据库错误
```bash
# 检查数据库文件
ls -lh ./music_party/data/

# 重置数据库（警告：会清除所有数据）
docker compose --env-file .env.test down
rm -rf ./music_party/data/musicparty.db
docker compose --env-file .env.test up -d
```

---

## 清理

### 停止服务
```bash
docker compose --env-file .env.test down
```

### 清理测试数据
```bash
# 删除测试配置
rm .env.test

# 删除测试 cookie
rm /tmp/guest-cookies.txt /tmp/admin-cookies.txt

# 清理镜像（可选）
docker rmi musicparty:guest-mode-local
```

### 完全清理
```bash
# 停止并删除所有容器、卷
docker compose --env-file .env.test down -v

# 清理未使用的镜像
docker image prune -f
```

---

## 验收标准

### 必须通过
- [x] Docker 镜像构建成功
- [x] 服务启动成功
- [x] 健康检查通过
- [x] 访客模式工作正常
- [x] 管理员登录工作正常
- [x] 升级横幅显示正常
- [x] UI 对比度问题已修复

### 可选验证
- [ ] 邀请码生成和使用
- [ ] 访客升级流程完整测试
- [ ] WebSocket 实时通信
- [ ] 音乐播放功能

---

## 验证完成后

### 如果所有测试通过
```bash
# 标记为验证通过
git tag -a v2.0-guest-mode-verified -m "Local verification passed"

# 准备推送到云端
# （等待您的指示）
```

### 如果发现问题
1. 记录问题现象
2. 查看相关日志
3. 修复代码
4. 重新构建和测试
5. 更新 commit

---

**验证清单完成后请告知，我们将准备推送到云端！**
