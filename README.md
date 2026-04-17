# Crawl Pic

一个可配置的帖子图片爬虫：
- 后端：Go + PostgreSQL
- 前端：React + TypeScript + Vite

## 功能

- 提交爬取任务（异步）
- 支持 BYR（需要登录态）自动翻页抓取
- 按 CSS Selector 抓取：帖子链接、帖子标题、帖子内图片
- 支持下一页翻页选择器
- 任务状态跟踪（pending/running/done/failed）
- 保存帖子内容与图片链接到 PostgreSQL（无图片的帖子不入库）

## 项目结构

- `backend/` Go API 与爬虫
- `frontend/` React 页面
- `docker-compose.yml` 本地 PostgreSQL

## 快速开始

### 1) 启动 PostgreSQL

```bash
docker compose up -d
```

查看状态：

```bash
docker compose ps
```

### 2) 启动后端

```bash
cd backend
cp .env.example .env
./install-air.sh
./dev.sh
```

后端默认地址：`http://localhost:8080`

说明：
- 后端源码启动，`air` 会监听代码变更并自动重启。
- 程序会自动读取 `backend/.env`，并连接 `docker compose` 启动的 PostgreSQL。
- 默认映射端口为本机 `5433`（容器内仍是 `5432`）。

### 2.1) 启动可登录的 Chrome（BYR 专用）

```bash
cd backend
./chrome-debug.sh
```

然后在这个 Chrome 窗口手动登录 `https://bbs.byr.cn/#!board/Friends`。

说明：
- `POST /api/cdp/jobs` 与 `POST /api/baidu-index/jobs` 也复用这个 Chrome 登录态。

### 3) 启动前端

```bash
cd frontend
npm install
npm run dev
```

前端默认地址：`http://localhost:5173`

### 4) 停止数据库

```bash
docker compose down
```

## API

### `POST /api/jobs`

请求体示例：

```json
{
  "siteName": "demo-site",
  "startUrls": ["https://example.com/forum"],
  "allowedDomains": ["example.com"],
  "postLinkSelector": "a.post-link",
  "nextPageSelector": "a.next",
  "imageSelector": "article img",
  "postTitleSelector": "h1",
  "maxListPages": 10,
  "maxPosts": 200,
  "requestTimeoutSecs": 20
}
```

### `GET /api/jobs/:id`
查看任务状态。

### `GET /api/jobs/:id/posts`
查看任务抓到的帖子。

### `GET /api/jobs/:id/photos`
查看任务抓到的图片链接。

### `POST /api/byr/jobs`
基于已登录 Chrome 抓 BYR 版面帖子内容与图片 URL（自动翻页）。

请求体示例：

```json
{
  "siteName": "byr-friends",
  "boardName": "Friends",
  "startPage": 1,
  "maxPages": 2000,
  "remoteDebugUrl": "http://127.0.0.1:9222"
}
```

### `POST /api/cdp/jobs`
基于已登录 Chrome（CDP）抓取单个网页正文与图片 URL。

请求体示例：

```json
{
  "siteName": "cdp-page",
  "startUrl": "https://example.com/article",
  "remoteDebugUrl": "http://127.0.0.1:9222",
  "pageReadySelector": "main.article",
  "contentSelector": "main.article",
  "imageSelector": "img",
  "titleSelector": "h1",
  "waitAfterLoadMs": 1800,
  "minImageBytes": 51200
}
```

### `POST /api/baidu-index/jobs`
基于已登录 Chrome（CDP）抓取百度指数趋势图（支持 `7d/30d/90d/180d/all`），将 canvas 转成 PNG 存到后端目录，并记录到任务图片结果中。

请求体示例：

```json
{
  "siteName": "baidu-index-openclaw",
  "keyword": "openclaw",
  "startUrl": "https://index.baidu.com/v2/main/index.html#/trend/openclaw?words=openclaw",
  "period": "90d",
  "remoteDebugUrl": "http://127.0.0.1:9222",
  "waitAfterLoadMs": 1800,
  "minImageBytes": 10240
}
```

输出说明：
- PNG 文件保存到 `ASSETS_DIR/baidu-index/`（默认 `backend/data/baidu-index/`）
- 文件名格式：`时间戳_搜索词_时间段_后缀.png`，例如 `20260417_151530_openclaw_90d_搜索.png`
- 可通过 `/assets/baidu-index/<filename>` 访问

## 注意

- 请仅抓取你有权限抓取的网站，遵守目标站点服务条款与 robots 约束。
- 当前存储的是图片 URL；如需下载图片文件，可在后端增加下载队列。
