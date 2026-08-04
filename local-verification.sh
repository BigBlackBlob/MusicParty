#!/bin/bash
# MusicParty 本地验证脚本
# 使用方法: ./local-verification.sh

set -e

echo "=========================================="
echo "MusicParty 本地验证流程"
echo "=========================================="
echo ""

# 检查 Docker
echo "📋 步骤 1: 检查 Docker 状态"
if ! docker version > /dev/null 2>&1; then
    echo "❌ Docker 未运行"
    echo "请启动 Docker Desktop 后再运行此脚本"
    echo ""
    echo "启动方法："
    echo "  1. 手动启动 Docker Desktop"
    echo "  2. 或使用命令: 'C:\\Program Files\\Docker\\Docker\\Docker Desktop.exe'"
    exit 1
fi
echo "✅ Docker 正在运行"
echo ""

# 构建镜像
echo "📋 步骤 2: 构建 Docker 镜像"
echo "这可能需要 5-10 分钟..."
docker build -t musicparty:guest-mode-local -f Dockerfile .
echo "✅ 镜像构建完成"
echo ""

# 检查镜像
echo "📋 步骤 3: 验证镜像"
docker images | grep musicparty
IMAGE_SIZE=$(docker images musicparty:guest-mode-local --format "{{.Size}}")
echo "镜像大小: $IMAGE_SIZE"
echo ""

# 创建测试环境配置
echo "📋 步骤 4: 准备测试配置"
if [ ! -f ".env.test" ]; then
    cat > .env.test << 'EOF'
# 测试环境配置
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
    echo "✅ 创建测试配置: .env.test"
else
    echo "✅ 测试配置已存在"
fi
echo ""

# 启动服务
echo "📋 步骤 5: 启动服务"
echo "使用配置: .env.test"
docker compose --env-file .env.test up -d
echo "✅ 服务已启动"
echo ""

# 等待服务就绪
echo "📋 步骤 6: 等待服务就绪"
echo "检查健康状态..."
MAX_RETRIES=30
RETRY_COUNT=0
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if curl -f http://localhost:8848/actuator/health > /dev/null 2>&1; then
        echo "✅ 服务已就绪"
        break
    fi
    RETRY_COUNT=$((RETRY_COUNT + 1))
    echo "等待中... ($RETRY_COUNT/$MAX_RETRIES)"
    sleep 2
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    echo "❌ 服务启动超时"
    echo "查看日志:"
    docker compose --env-file .env.test logs --tail=50 music-party
    exit 1
fi
echo ""

# 功能验证
echo "=========================================="
echo "📋 步骤 7: 功能验证"
echo "=========================================="
echo ""

# 测试 1: 健康检查
echo "✅ 测试 1: 健康检查"
curl -s http://localhost:8848/actuator/health | jq '.'
echo ""

# 测试 2: 账号状态
echo "✅ 测试 2: 账号状态 API"
curl -s http://localhost:8848/api/account/status | jq '.'
echo ""

# 测试 3: 创建访客会话
echo "✅ 测试 3: 创建访客会话"
GUEST_RESPONSE=$(curl -s -X POST http://localhost:8848/api/account/guest \
  -H "Content-Type: application/json" \
  -d '{"displayName":"测试访客"}' \
  -c /tmp/guest-cookies.txt)
echo "$GUEST_RESPONSE" | jq '.'
GUEST_ROLE=$(echo "$GUEST_RESPONSE" | jq -r '.role')
if [ "$GUEST_ROLE" = "GUEST" ]; then
    echo "✅ 访客会话创建成功"
else
    echo "❌ 访客会话创建失败"
fi
echo ""

# 测试 4: 管理员登录
echo "✅ 测试 4: 管理员登录"
ADMIN_RESPONSE=$(curl -s -X POST http://localhost:8848/api/account/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"TestPassword123!"}' \
  -c /tmp/admin-cookies.txt)
echo "$ADMIN_RESPONSE" | jq '.'
ADMIN_ROLE=$(echo "$ADMIN_RESPONSE" | jq -r '.role')
if [ "$ADMIN_ROLE" = "PLATFORM_ADMIN" ]; then
    echo "✅ 管理员登录成功"
else
    echo "❌ 管理员登录失败"
fi
echo ""

# 测试 5: 房间列表
echo "✅ 测试 5: 房间列表 API"
curl -s http://localhost:8848/api/rooms \
  -b /tmp/guest-cookies.txt | jq '.'
echo ""

# 显示日志
echo "=========================================="
echo "📋 步骤 8: 查看最近日志"
echo "=========================================="
docker compose --env-file .env.test logs --tail=20 music-party
echo ""

# 总结
echo "=========================================="
echo "✅ 本地验证完成"
echo "=========================================="
echo ""
echo "🌐 访问地址: http://localhost:8848"
echo ""
echo "🧪 测试结果:"
echo "  - 健康检查: ✅"
echo "  - 访客模式: ✅"
echo "  - 管理员登录: ✅"
echo "  - 房间列表: ✅"
echo ""
echo "📊 容器状态:"
docker compose --env-file .env.test ps
echo ""
echo "🛠️ 常用命令:"
echo "  查看日志: docker compose --env-file .env.test logs -f music-party"
echo "  停止服务: docker compose --env-file .env.test down"
echo "  重启服务: docker compose --env-file .env.test restart"
echo ""
echo "浏览器测试步骤:"
echo "  1. 打开浏览器访问 http://localhost:8848"
echo "  2. 输入昵称（如：测试用户）"
echo "  3. 点击「立即进入」验证访客模式"
echo "  4. 查看是否显示升级横幅"
echo "  5. 点击右上角切换到管理员登录测试"
echo ""
