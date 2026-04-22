# Owl Dictionary（中文说明）

Owl 是一个支持 **MDX / MDD** 词典文件的 Web 词典应用。当前架构为：**Go 后端 + 嵌入式 React 前端 + 单二进制部署**。

## 技术栈

- 后端：Go + Echo v5 + ent + SQLite（`github.com/lib-x/entsqlite`）
- 词典解析引擎：`github.com/lib-x/mdx v0.1.9`
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

```bash
cp .env.example .env
docker compose up --build
```

默认地址：
- `http://localhost:8080`

说明：
- 前端由 Go 服务端直接提供
- 数据通过 `owl_data` volume 持久化

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
