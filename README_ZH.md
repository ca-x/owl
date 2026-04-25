# Owl Dictionary（中文说明）

[English](README.md)

Owl 是一个自托管的 **MDX / MDD Web 词典**。它面向个人查词、公开词典共享、用户私有词典管理，以及 AI 客户端通过 MCP 查询词典的场景。

---

## 用户可以做什么

### 查词与阅读

- 在浏览器中直接查询 MDX 词典。
- 未登录用户可以查询已启用的**公开词典**。
- 登录用户可以查询公开词典，以及自己上传的私有词典。
- 支持查询全部可用词典，也可以筛选到某一本词典。
- 支持搜索建议和键盘导航。
- 支持点击词条内部链接，继续跳转查询相关词。
- 支持渲染 MDX 返回的 HTML 内容。
- 支持通过后端读取配套 MDD 中的图片、音频、CSS、字体和其他媒体资源。
- 词条中有音频资源时可以直接播放。
- 支持复制纯文本释义，方便粘贴到笔记或其他工具中。
- 支持最近搜索，保留数量可以在管理界面配置。

### 更舒服的输出体验

- 查询页面同时适配桌面端和移动端。
- 移动端提供更紧凑的词典筛选控件。
- 移动端搜索后会直接定位到结果区域，避免最近搜索挡住结果。
- 最佳匹配结果会被突出显示，其他匹配结果在下方继续展示。
- 查询结果会显示公开 / 私有来源标识。
- 支持多种主题，包括复古风格、深色主题、黑白主题等。
- 阅读字体可以切换为 sans / serif / mono / 自定义字体。

### 个人偏好

- 切换界面语言：简体中文 / English。
- 切换视觉主题。
- 设置阅读字体模式。
- 上传并使用共享自定义字体。
- 设置显示名称和头像。
- 设置最近搜索保留数量。

---

## 词典管理功能

### 词典维护

- 在网页中上传 `.mdx` 文件和可选的 `.mdd` 文件。
- 刷新单本词典，用于重新加载或补齐资源。
- 刷新整个词典库，用于重新扫描挂载目录。
- 启用 / 停用词典。
- 设置词典为公开 / 私有。
- 删除词典。
- 在界面中查看词典文件状态：
  - `ok`
  - `missing_mdx`
  - `missing_mdd`
  - `missing_all`
- 查看结构化刷新报告：
  - discovered
  - updated
  - skipped
  - failed

### 词典文件识别规则

- 同 basename 的 `.mdx + .mdd` 会被视为同一套词典。
- 如果先上传 MDX，之后再补充同名 MDD，可以通过刷新重新发现资源。
- 挂载的词典目录会被递归扫描。
- 如果不挂载外部目录，也可以只使用网页上传模式。

### 站点级管理

管理员可以设置：

- 是否开放新用户注册。
- 网站 footer 额外信息。
- 版权信息。

默认情况下，footer 信息为空时不会显示。

---

## MCP 支持

Owl 内置基于 SSE 的 MCP 服务，方便 AI 客户端调用词典。

### Endpoint

```text
/api/mcp/sse
```

认证方式是每个用户自己的 MCP Token：

```text
Authorization: Bearer <MCP_TOKEN>
```

临时测试也可以使用 URL 参数：

```text
/api/mcp/sse?token=<MCP_TOKEN>
```

初次 SSE 连接必须携带 Token；连接建立后，SDK 后续 POST 请求会通过 MCP session 继续通信。

### MCP 工具

- `list_dictionaries`
  - 列出当前 token 用户可访问的词典。
  - 范围：已启用的公开词典 + 当前用户自己的私有词典。

- `search_dictionary`
  - 查询当前 token 用户可访问的词典。
  - 必填：`query`
  - 可选：`dictionary_id` 或 `dictionary_name`
  - 如果不指定词典，则按 Web 查询相同范围搜索全部可访问词典。

### Token 管理

每个用户都可以在管理界面维护自己的 MCP Token：

- 保存自定义 Token
- 生成随机 Token
- 删除 / 撤销 Token
- 打开使用说明弹窗查看接入方式

Token 只会以 hash 形式存储。生成后请立即复制，之后界面只显示首尾提示。

---

## 技术概览

### 技术栈

- 后端：Go + Echo v5 + ent + SQLite（`github.com/lib-x/entsqlite`）
- 词典引擎：`github.com/lib-x/mdx`
- MCP 服务：`github.com/modelcontextprotocol/go-sdk`
- 前端：React + Vite + TypeScript
- 可选搜索缓存 / 索引：Redis + RediSearch
- 部署方式：单 Go 服务 / 单 Docker 镜像
- 前端生产资源：通过 `go:embed` 嵌入 Go 服务
- 自动化：GitHub Actions 构建 CI、发布二进制和 Docker 镜像

### 搜索后端行为

Owl 可以不依赖 Redis 运行，此时使用本地 MDX 搜索 / 索引能力。

配置 Redis 后：

- exact / prefix 索引可以写入 Redis
- fuzzy 查询可以使用 RediSearch
- 自动补全结果由后端聚合
- 如果 RediSearch 不可用，会自动回退到内存 fuzzy store

---

## Docker 部署

仓库提供两个 Docker Compose 文件：

- `docker-compose.yml`：最简单部署，不启用 Redis
- `docker-compose.redis.yml`：启用 Redis + RediSearch

### 方案一：不启用 Redis

```bash
cp .env.example .env
# 先修改 OWL_JWT_SECRET 和管理员账号密码
docker compose -f docker-compose.yml pull
docker compose -f docker-compose.yml up -d
```

默认地址：

```text
http://localhost:8080
```

该模式会启动：

- Owl：`http://localhost:8080`
- SQLite：保存在持久化 volume `owl_data` 中
- 上传词典：保存在 `/app/data/uploads`
- 不依赖 Redis

### 方案二：启用 Redis + RediSearch

如果你希望启用 Redis 前缀 / 精确索引和 RediSearch 模糊查询，可以使用：

```bash
cp .env.example .env
# 先修改 OWL_JWT_SECRET 和管理员账号密码
docker compose -f docker-compose.redis.yml pull
docker compose -f docker-compose.redis.yml up -d
```

该模式会启动：

- Owl：`http://localhost:8080`
- Redis Stack Server，用于 Redis + RediSearch
- SQLite 和上传文件保存在 `owl_data`
- Redis 数据保存在 `owl_redis`

Compose 文件默认使用发布镜像：`czyt/owl:latest`。

---

## 部署说明

### 挂载已有词典目录

如果你本机已经有很多 `.mdx` / `.mdd` 文件，可以把宿主机目录挂载到 `OWL_LIBRARY_DIR`。

示例覆盖：

```yaml
services:
  owl:
    environment:
      OWL_LIBRARY_DIR: /app/library
    volumes:
      - owl_data:/app/data
      - ./dicts:/app/library
```

启动后：

1. 登录
2. 打开 **管理**
3. 点击 **刷新词典库**

Owl 会递归扫描目录，并自动把 `name.mdx` 和 `name.mdd` 识别为同一套词典。

### 纯网页上传模式

如果你不想挂载外部词典目录，可以保持：

```text
OWL_LIBRARY_DIR=/app/data/uploads
```

这样词典就可以完全通过网页上传和管理。

### 首次启动检查清单

1. 打开 `http://localhost:8080`。
2. 使用初始化管理员账号登录。
3. 上传一本测试词典，或挂载词典目录后刷新词典库。
4. 根据需要设置词典公开 / 私有。
5. 回到首页确认可以正常查词。
6. 可选：配置注册开关、footer、字体和 MCP Token。

### 升级说明

升级 Owl 时，请使用与你启动时相同的 compose 文件。

不启用 Redis：

```bash
git pull
docker compose -f docker-compose.yml down
docker compose -f docker-compose.yml pull
docker compose -f docker-compose.yml up -d
```

启用 Redis：

```bash
git pull
docker compose -f docker-compose.redis.yml down
docker compose -f docker-compose.redis.yml pull
docker compose -f docker-compose.redis.yml up -d
```

SQLite 数据、上传词典和 Redis 数据都会保留在 Docker volume 中，除非你手动删除 volume。

---

## 本地开发

### 后端

```bash
cd backend
GOPROXY=https://goproxy.cn,direct GOSUMDB=off go test ./...
GOPROXY=https://goproxy.cn,direct GOSUMDB=off go vet ./...
GOPROXY=https://goproxy.cn,direct GOSUMDB=off go run ./cmd/server
```

后端默认地址：

```text
http://localhost:8080
```

### 前端

```bash
cd frontend
pnpm install
pnpm lint
pnpm build
pnpm dev
```

前端开发地址：

```text
http://localhost:3000
```

生产风格本地运行时，`pnpm build` 会把前端构建产物写入 `backend/web/dist`，Go 服务通过嵌入资源提供页面。Vite 开发服务会把 `/api` 代理到后端。

---

## 环境变量

完整列表见 `.env.example`。

### 核心服务

- `OWL_PORT`
- `OWL_FRONTEND_ORIGIN`
- `OWL_JWT_SECRET`
- `OWL_DATA_DIR`
- `OWL_UPLOADS_DIR`
- `OWL_LIBRARY_DIR`
- `OWL_DB_PATH`

### 初始化管理员与注册

- `OWL_BOOTSTRAP_ADMIN`
- `OWL_ADMIN_USERNAME`
- `OWL_ADMIN_PASSWORD`
- `OWL_ALLOW_REGISTER`

### Redis / RediSearch

- `OWL_REDIS_ADDR`
- `OWL_REDIS_PASSWORD`
- `OWL_REDIS_DB`
- `OWL_REDIS_KEY_PREFIX`
- `OWL_REDIS_PREFIX_MAX_LEN`
- `OWL_REDIS_SEARCH_ENABLED`
- `OWL_REDIS_SEARCH_KEY_PREFIX`

### 音频资源

- `OWL_AUDIO_CACHE_DIR`
- `FFMPEG_BIN`

---

## 调试接口

### 查看搜索后端状态

游客范围：

```bash
curl http://localhost:8080/api/public/search-backends
```

登录范围：

```bash
curl -H 'Authorization: Bearer <token>' http://localhost:8080/api/debug/search-backends
```

重点字段含义：

- `fuzzy_backend: redisearch`：正在使用 RediSearch
- `fuzzy_backend: memory-fuzzy`：处于回退模式
- `prefix_backend: redis-prefix`：Redis 前缀索引已生效
