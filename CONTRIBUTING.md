# 贡献指南

感谢你对 MusicParty 项目的关注！我们欢迎各种形式的贡献。

---

## 📋 目录

1. [行为准则](#行为准则)
2. [如何贡献](#如何贡献)
3. [开发环境搭建](#开发环境搭建)
4. [代码规范](#代码规范)
5. [提交 Pull Request](#提交-pull-request)
6. [报告 Bug](#报告-bug)
7. [功能建议](#功能建议)

---

## 🤝 行为准则

参与本项目即表示你同意遵守以下准则：

- 尊重所有贡献者
- 使用友好和包容的语言
- 接受建设性批评
- 关注对社区最有利的事情
- 对其他社区成员表示同理心

---

## 🎯 如何贡献

### 贡献类型

- **代码贡献**: 修复 Bug、添加新功能、性能优化
- **文档贡献**: 改进文档、添加示例、翻译
- **测试贡献**: 编写测试用例、报告 Bug
- **设计贡献**: UI/UX 改进、图标设计
- **社区贡献**: 回答问题、帮助新用户

---

## 🛠️ 开发环境搭建

### 前置要求

- **Go**: 1.26+
- **Node.js**: 20+
- **Docker**: 最新版本
- **Git**: 最新版本

### 1. Fork 并克隆仓库

```bash
# Fork 本仓库到你的 GitHub 账号

# 克隆你 fork 的仓库
git clone https://github.com/YOUR_USERNAME/MusicParty.git
cd MusicParty

# 添加上游仓库
git remote add upstream https://github.com/BigBlackBlob/MusicParty.git
```

### 2. 后端开发环境

```bash
cd backend-go

# 安装依赖
go mod download

# 运行测试
go test ./...

# 本地运行
go run ./cmd/musicparty
```

### 3. 前端开发环境

```bash
cd music-party-web

# 安装依赖
npm install

# 运行开发服务器
npm run dev

# 运行测试
npm run test:run

# 运行 Lint
npm run lint
```

### 4. 完整环境（Docker Compose）

```bash
# 启动所有服务
docker compose -f docker-compose.build.yml up --build

# 查看日志
docker compose logs -f music-party
```

---

## 📝 代码规范

### Go 代码规范

**遵循官方 Go 代码风格**：
- 使用 `gofmt` 格式化代码
- 使用 `golangci-lint` 检查代码质量
- 遵循 [Effective Go](https://go.dev/doc/effective_go) 指南

**命名约定**：
```go
// ✅ 好的命名
type UserService struct { }
func (s *UserService) CreateSession(ctx context.Context, name string) error

// ❌ 不好的命名
type userservice struct { }
func (s *userservice) create_session(ctx context.Context, name string) error
```

**错误处理**：
```go
// ✅ 好的错误处理
result, err := service.DoSomething(ctx)
if err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// ❌ 不好的错误处理
result, _ := service.DoSomething(ctx)  // 忽略错误
```

**测试**：
```go
// 测试文件命名: *_test.go
// 测试函数命名: TestXxx
func TestCreateGuestSession(t *testing.T) {
    t.Run("creates valid session", func(t *testing.T) {
        // 测试逻辑
    })
}
```

### Vue 代码规范

**遵循 Vue 3 风格指南**：
- 使用 Composition API
- 组件文件使用 PascalCase 命名
- 使用 `<script setup>` 语法

**组件结构**：
```vue
<template>
  <!-- 模板 -->
</template>

<script setup>
// 导入
import { ref, computed } from 'vue';

// 组合式函数
const count = ref(0);
const double = computed(() => count.value * 2);

// 方法
const increment = () => {
  count.value++;
};
</script>

<style scoped>
/* 样式 */
</style>
```

**命名约定**：
```javascript
// ✅ 好的命名
const isVisible = ref(true);
const userName = ref('');
const handleClick = () => { };

// ❌ 不好的命名
const visible = ref(true);  // 布尔值应该有 is/has 前缀
const user_name = ref('');  // 使用驼峰命名
const click = () => { };    // 事件处理应该有 handle 前缀
```

### 通用规范

**Commit 消息格式**：
```
type(scope): subject

body (可选)

footer (可选)
```

**Type 类型**：
- `feat`: 新功能
- `fix`: Bug 修复
- `docs`: 文档更新
- `style`: 代码格式（不影响代码运行）
- `refactor`: 重构
- `test`: 测试相关
- `chore`: 构建过程或辅助工具的变动

**示例**：
```
feat(auth): implement guest mode

Add guest session creation without invite code.
Guests can listen to music but cannot create rooms.

Closes #123
```

---

## 🔄 提交 Pull Request

### 1. 创建功能分支

```bash
# 更新你的本地 main 分支
git checkout main
git pull upstream main

# 创建新分支
git checkout -b feature/your-feature-name
```

### 2. 编写代码

- 遵循代码规范
- 编写测试用例
- 更新相关文档

### 3. 提交代码

```bash
# 添加修改
git add .

# 提交（使用规范的 commit 消息）
git commit -m "feat(scope): description"

# 推送到你的 fork
git push origin feature/your-feature-name
```

### 4. 创建 Pull Request

1. 访问你的 fork 仓库
2. 点击 "New Pull Request"
3. 选择你的分支
4. 填写 PR 描述：
   - 简要说明你的改动
   - 关联相关 Issue（如果有）
   - 说明测试情况
   - 附上截图（如果是 UI 改动）

### 5. PR 审查

- 等待维护者审查
- 根据反馈修改代码
- 保持 PR 更新（rebase 到最新 main）

### PR 检查清单

- [ ] 代码遵循项目规范
- [ ] 所有测试通过
- [ ] 添加了必要的测试
- [ ] 更新了相关文档
- [ ] Commit 消息清晰明确
- [ ] 没有不相关的代码变更

---

## 🐛 报告 Bug

### Bug 报告模板

```markdown
**Bug 描述**
简要描述遇到的问题

**复现步骤**
1. 进入 '...'
2. 点击 '...'
3. 看到错误

**预期行为**
描述你期望发生什么

**实际行为**
描述实际发生了什么

**环境信息**
- 浏览器: [e.g. Chrome 120]
- 操作系统: [e.g. Windows 11]
- MusicParty 版本: [e.g. v2.0]

**截图**
如果适用，添加截图帮助说明问题

**额外信息**
任何其他相关信息
```

### 提交 Bug Report

1. 访问 [Issues](https://github.com/BigBlackBlob/MusicParty/issues)
2. 点击 "New Issue"
3. 选择 "Bug Report"
4. 填写模板
5. 提交

---

## 💡 功能建议

### 功能请求模板

```markdown
**功能描述**
简要描述你想要的功能

**使用场景**
描述为什么需要这个功能，解决什么问题

**建议的实现方式**
如果有想法，描述你认为应该如何实现

**替代方案**
是否考虑过其他解决方案

**额外信息**
任何其他相关信息或截图
```

---

## 📚 开发资源

### 文档

- [架构文档](./docs/architecture.md)
- [运维手册](./docs/operations.md)
- [API 文档](./docs/api/)

### 技术栈

**后端**:
- [Go 官方文档](https://go.dev/doc/)
- [Chi Router](https://github.com/go-chi/chi)
- [SQLite](https://www.sqlite.org/docs.html)

**前端**:
- [Vue 3 文档](https://vuejs.org/)
- [Pinia 文档](https://pinia.vuejs.org/)
- [Tailwind CSS](https://tailwindcss.com/docs)

### 常用命令

```bash
# 后端
go test ./...                    # 运行所有测试
go test -v ./internal/domain/... # 运行指定包测试
go test -race ./...              # 检测数据竞争
golangci-lint run                # 运行 linter

# 前端
npm run dev                      # 开发服务器
npm run build                    # 生产构建
npm run test:run                 # 运行测试
npm run lint                     # 运行 linter
```

---

## 🎓 学习资源

### 新手建议

1. **先从小改动开始**：文档修正、测试用例
2. **阅读现有代码**：理解项目结构和风格
3. **参与讨论**：在 Issues 中提问和讨论
4. **寻求帮助**：不要害怕问问题

### 推荐阅读

- [Git 工作流](https://www.atlassian.com/git/tutorials/comparing-workflows)
- [如何编写 Git Commit 消息](https://chris.beams.io/posts/git-commit/)
- [代码审查最佳实践](https://google.github.io/eng-practices/review/)

---

## 🙏 致谢

感谢所有贡献者让 MusicParty 变得更好！

特别感谢：
- 上游 MusicParty 项目的原作者
- 所有提交 Issue 和 PR 的贡献者
- 在社区中帮助他人的成员

---

## 📞 联系方式

- **GitHub Issues**: 技术问题和 Bug 报告
- **Discussions**: 功能讨论和一般问题
- **Email**: 仅用于敏感问题

---

**再次感谢你的贡献！** 🎉
