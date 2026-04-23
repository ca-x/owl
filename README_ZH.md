# Owl Dictionary（中文说明）

[English](README.md)

Owl 是一个支持 **MDX / MDD** 词典文件的 Web 词典应用。当前架构为：**Go 后端 + 嵌入式 React 前端 + 单二进制部署**。

## 技术栈

- 后端：Go + Echo v5 + ent + SQLite（`github.com/lib-x/entsqlite`）
- 词典解析引擎：`github.com/lib-x/mdx v0.1.11`
- 前端：React + Vite + TypeScript + pnpm
- 部署：单 Go 服务 / 单 Docker 镜像
- 自动化：GitHub Actions（CI、二进制发布、Docker 构建）

## 核心能力

### 查询能力

- 未登录用户可查询：**已启用的公开词典**
- 已登录用户可查询：
  - 所有已启用的公开词典
  - 自己的私有词典
- 查询结果支持：
  - Best match 最佳结果
  - 同词跨词典对比
  - Public / Private 来源分组
  - 搜索建议与键盘导航
- MDX 返回的 HTML 会直接渲染
- MDD 中的图片、CSS、字体等资源通过后端路由提供

### 词典维护能力

- 上传 MDX 与可选的 MDD 文件
- 同 basename 的 `.mdx + .mdd` 视为同一词典
- 先传 MDX、后补 MDD 后可通过刷新重新发现资源
- 支持递归扫描挂载目录中的词典文件
- 词典状态会在 UI 中显示：
  - `ok`
  - `missing_mdx`
  - `missing_mdd`
  - `missing_all`
- 维护动作包括：
  - 刷新单个词典
  - 刷新整个词典库
  - 启用 / 停用
  - 公开 / 私有切换
  - 删除
- 刷新会返回结构化维护报告：
  - discovered
  - updated
  - skipped
  - failed

### 用户偏好

每个用户的偏好设置持久化在后端：
- 语言：`zh-CN` / `en`
- 主题：`system` / `light` / `dark` / `sepia`
- 阅读字体：`sans` / `serif` / `mono` / `custom`
- 支持上传自定义字体

## 本地开发

### 后端

```bash
cd backend
GOPROXY=https://goproxy.cn,direct GOSUMDB=off go test ./...
GOPROXY=https://goproxy.cn,direct GOSUMDB=off go vet ./...
GOPROXY=https://goproxy.cn,direct GOSUMDB=off go run ./cmd/server
```

默认地址：
- `http://localhost:8080`

### 前端

```bash
cd frontend
pnpm install
pnpm lint
pnpm build
pnpm dev
```

前端开发地址：
- `http://localhost:3000`

说明：
- `pnpm build` 会把构建产物写到 `backend/web/dist`
- Go 服务端通过 `go:embed` 直接提供前端页面
- 开发模式下 Vite 仍会代理 `/api` 到后端

## Docker 部署

当前仓库只保留两个 Docker Compose 文件：
- `docker-compose.yml` → 不启用 Redis，最简单
- `docker-compose.redis.yml` → 启用 Redis + RediSearch

### 方案一：不启用 Redis

```bash
cp .env.example .env
# 先修改 OWL_JWT_SECRET 和管理员账号密码
docker compose -f docker-compose.yml pull
docker compose -f docker-compose.yml up -d
```

默认地址：
- `http://localhost:8080`

启动后会得到：
- Owl：`http://localhost:8080`
- SQLite：存放在持久化 volume `owl_data` 中
- 上传词典：存放在 `/app/data/uploads`
- 使用内存 fuzzy search，不依赖 Redis

### 方案二：启用 Redis + RediSearch

如果你希望启用 Redis 精确 / 前缀索引，以及 RediSearch 模糊搜索，推荐这样启动：

```bash
cp .env.example .env
# 先修改 OWL_JWT_SECRET 和管理员账号密码
docker compose -f docker-compose.redis.yml pull
docker compose -f docker-compose.redis.yml up -d
```

这个模式下通常会是：
- exact/prefix：Redis
- fuzzy：RediSearch
- 如果模块不可用：自动回退到内存 fuzzy search

说明：
- 前端由 Go 服务端直接提供
- 数据通过 `owl_data` volume 持久化
- Redis 数据通过 `owl_redis` volume 持久化
- 仓库内的 compose 直接使用镜像 `czyt/owl:latest`

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

Owl 会递归扫描这个目录，并自动把同名的 `name.mdx` 和 `name.mdd` 识别成同一套词典。

### 纯网页上传模式

如果你不想挂载外部词典目录，可以保持：
- `OWL_LIBRARY_DIR=/app/data/uploads`

这样词典就全部通过网页上传管理。

### 首次启动检查清单

容器启动后建议按这个顺序检查：
1. 打开 `http://localhost:8080`
2. 使用初始化管理员账号登录
3. 上传一本测试词典，或者挂载后刷新词典库
4. 在 **管理** 页面设置公开 / 私有
5. 回到首页确认能正常查词

### 如何确认当前实际使用的搜索后端

游客范围：

```bash
curl http://localhost:8080/api/public/search-backends
```

登录范围：

```bash
curl -H 'Authorization: Bearer <token>' http://localhost:8080/api/debug/search-backends
```

重点字段含义：
- `fuzzy_backend: redisearch` → 当前正在使用 RediSearch
- `fuzzy_backend: memory-fuzzy` → 当前处于回退模式
- `prefix_backend: redis-prefix` → Redis 前缀索引已生效

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

SQLite 数据、上传词典和 Redis 数据都会保留在 Docker volume 中，除非你手动删除。

## 重要环境变量

详见 `.env.example`。

常用项：
- `OWL_JWT_SECRET`
- `OWL_BOOTSTRAP_ADMIN`
- `OWL_ADMIN_USERNAME`
- `OWL_ADMIN_PASSWORD`
- `OWL_DATA_DIR`
- `OWL_UPLOADS_DIR`
- `OWL_LIBRARY_DIR`
- `OWL_DB_PATH`
- `OWL_REDIS_ADDR`（可选）
- `OWL_REDIS_PASSWORD`
- `OWL_REDIS_DB`
- `OWL_REDIS_KEY_PREFIX`
- `OWL_REDIS_PREFIX_MAX_LEN`
- `OWL_REDIS_SEARCH_ENABLED`
- `OWL_REDIS_SEARCH_KEY_PREFIX`

## 可选 Redis 索引缓存

Owl 可以选择使用 Redis 作为 MDX 精确 / 前缀索引缓存层。

当前行为：
- 配置 Redis 后，Owl 会优先尝试基于 RediSearch 的模糊检索
- exact/prefix suggestion 索引也会存入 Redis
- 自动补全分组结果由后端聚合
- 如果 RediSearch 模块不可用，会自动回退到内存 mdx fuzzy store

启用时可设置：
- `OWL_REDIS_ADDR=redis:6379`
- 可选 `OWL_REDIS_PASSWORD` / `OWL_REDIS_DB`
- 可选 `OWL_REDIS_KEY_PREFIX` / `OWL_REDIS_PREFIX_MAX_LEN`
- `OWL_REDIS_SEARCH_ENABLED=true`
- 可选 `OWL_REDIS_SEARCH_KEY_PREFIX`

如果要直接使用仓库内置的 Redis 部署：

```bash
docker compose -f docker-compose.redis.yml pull
docker compose -f docker-compose.redis.yml up -d
```

调试接口：
- 游客范围：`GET /api/public/search-backends`
- 登录范围：`GET /api/debug/search-backends`

这些接口会展示当前每本词典实际使用的是：
- `redisearch`
- `memory-fuzzy`
- `redis-prefix`

## 默认管理员账号

如果 `OWL_BOOTSTRAP_ADMIN=true`，启动时会自动创建管理员账号（若不存在）。

默认示例：
- 用户名：`admin`
- 密码：`admin123456`

上线前请务必修改。

## API 概览

### 公共接口
- `GET /api/health`
- `GET /api/public/dictionaries`
- `GET /api/public/search?q=word&dict=id`
- `GET /api/public/suggest?q=prefix&dict=id`
- `GET /api/public/search-backends`
- `GET /api/public/dictionaries/:id/resource/*`
- `POST /api/auth/register`
- `POST /api/auth/login`

### 登录后接口
- `POST /api/auth/logout`
- `GET /api/me`
- `GET /api/preferences`
- `PUT /api/preferences`
- `POST /api/preferences/font`
- `GET /api/preferences/font`
- `GET /api/dictionaries`
- `POST /api/dictionaries/upload`
- `PATCH /api/dictionaries/:id`
- `PATCH /api/dictionaries/:id/public`
- `POST /api/dictionaries/:id/refresh`
- `POST /api/dictionaries/refresh`
- `DELETE /api/dictionaries/:id`
- `GET /api/dictionaries/:id/resource/*`
- `GET /api/search?q=word&dict=id`
- `GET /api/suggest?q=prefix&dict=id`
- `GET /api/debug/search-backends`

## CI / 发布自动化

`.github/workflows/` 中包含：

- `ci.yml`
  - 前端安装 / lint / build（产物写入嵌入目录）
  - 后端 test / vet
  - 单镜像 Docker build 验证
- `binary.yml`
  - tag 触发二进制构建
  - 上传 draft release 资产
- `docker.yml`
  - tag 触发单镜像多架构 Docker 构建与推送

## 当前本地已验证内容

已验证：
- `go test ./...`
- `go vet ./...`
- `go build ./cmd/server`
- `pnpm lint`
- `pnpm build`
- 使用真实 OALD9 样本验证：图片资源可正常返回；`@@LINK` 不再泄漏到页面文本

未在当前环境完成：
- Docker 运行时完整验证（当前会话环境受限制）
- 多套真实词典资源包的完整端到端回归


## 音频兼容性

Owl 现在会检测 Ogg Speex / `.spx` 词典音频；当系统内存在 `ffmpeg` 时，会在后端按需透明转码为浏览器可播放的 MP3，并把结果缓存到配置的音频缓存目录。


新增环境变量：
- `OWL_AUDIO_CACHE_DIR`
- `FFMPEG_BIN`
