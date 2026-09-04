# Cyberlife

当前实现覆盖工程方案中的**第一、二阶段**：身份与人生空间基础，以及 Now（现在）页的最小记录闭环。

## 已实现

- Go/Gin API 骨架与 Vue 3 管理界面。
- `global.db` 初始化：Admin、Writer、Life、ReaderKey、Session、月库索引。
- 首次启动使用 `CYBERLIFE_ADMIN_PASSWORD` 初始化唯一管理员；密码只保存 Argon2id 哈希。
- Admin 创建 Writer，服务端生成并仅在响应中返回一次主密钥，同时创建对应人生和当月数据库。
- Admin 为指定人生创建、列出、作废 ReaderKey；作废即提升 key version，使既有会话失效。
- Writer/Reader 主密钥登录、Admin 独立密码登录、HttpOnly Cookie 会话、登出。
- SQLite WAL、外键、busy timeout 与 life 月库初始化。
- Now 垂直切片：书写者可创建心情标签、记录心情/身体、自动保存当日日记（独立草稿与正式内容）、添加及完成当天任务。
- 日记附件提供书写者上传入口：单文件最大 20MB、服务端 UUID 重命名、月库保存元数据；下载授权、MIME 魔数验证与独立附件权限将在 ACL 阶段实现。
- 现在页通过真实 API 读取上述数据；阅读者内容读取仍待第三阶段 ACL 完成后开放。

> 当前不包含附件上传、删除段落备份、日历跨日任务、条目级权限预设、绝密层、评论和三屏完整导航；这些功能将在后续阶段建立在已完成的身份/存储边界上。

## 启动

需要 Go 1.24+、Node.js 22+、pnpm。

```powershell
# API
$env:CYBERLIFE_ADMIN_PASSWORD = "设置一个长且唯一的管理员密码"
$env:CYBERLIFE_DATA_DIR = "E:\CyberlifeData" # 建议仓库外
cd server
go mod tidy
go run ./cmd/cyberlife

# 单独终端启动 Web
cd web
pnpm install
pnpm dev
```

打开 Vite 给出的本地地址（默认 `http://127.0.0.1:5173`）。Web 开发服务器将 `/api` 代理至 `127.0.0.1:8080`。

## 首批 API

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/admin/auth/login` | Admin 密码登录 |
| POST | `/api/v1/auth/key-login` | Writer/Reader 密钥登录 |
| POST | `/api/v1/auth/logout` | 当前会话登出 |
| GET | `/api/v1/auth/me` | 当前身份与能力 |
| GET/POST | `/api/v1/admin/writers` | 管理员列出/创建 Writer |
| GET/POST | `/api/v1/admin/writers/:lifeID/reader-keys` | 列出/创建阅读密钥 |
| POST | `/api/v1/admin/reader-keys/:id/revoke` | 作废阅读密钥 |
| GET | `/api/v1/now` | 书写者读取今日记录 |
| GET/POST | `/api/v1/now/mood-tags` | 列出/创建心情标签 |
| POST | `/api/v1/now/moods` | 按服务端标签值记录心情 |
| POST | `/api/v1/now/body` | 记录身体评分 |
| PUT | `/api/v1/now/diary/draft` | 保存独立日记草稿 |
| PUT | `/api/v1/now/diary` | 保存当日日记 |
| POST | `/api/v1/now/diary/attachments` | 上传当日日记附件 |
| POST | `/api/v1/now/tasks` | 创建当天任务 |
| POST | `/api/v1/now/tasks/:id/done` | 更新任务完成状态 |

## 安全边界

- 不提交 `runtime-data/`、数据库、上传文件、`.env` 或密钥。
- 生产环境通过 HTTPS 反向代理，并设置 `CYBERLIFE_SECURE_COOKIES=true`。
- 主/阅读密钥只在创建响应中返回；数据库只保存 Argon2id 哈希。
- 当前密钥查找会遍历现有哈希，适合个人内网低数量实例；规模化前应引入不可逆 key lookup verifier，避免登录随 key 数线性增长。
