# Cyberlife

当前实现覆盖工程方案中的**第一阶段**：身份与人生空间基础。

## 已实现

- Go/Gin API 骨架与 Vue 3 管理界面。
- `global.db` 初始化：Admin、Writer、Life、ReaderKey、Session、月库索引。
- 首次启动使用 `CYBERLIFE_ADMIN_PASSWORD` 初始化唯一管理员；密码只保存 Argon2id 哈希。
- Admin 创建 Writer，服务端生成并仅在响应中返回一次主密钥，同时创建对应人生和当月数据库。
- Admin 为指定人生创建、列出、作废 ReaderKey；作废即提升 key version，使既有会话失效。
- Writer/Reader 主密钥登录、Admin 独立密码登录、HttpOnly Cookie 会话、登出。
- SQLite WAL、外键、busy timeout 与 life 月库初始化。

> 这一阶段不包含日记、心情、身体、任务、ACL 条目预设或三屏内容功能；它们将在后续阶段建立在已完成的身份/存储边界上。

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

## 安全边界

- 不提交 `runtime-data/`、数据库、上传文件、`.env` 或密钥。
- 生产环境通过 HTTPS 反向代理，并设置 `CYBERLIFE_SECURE_COOKIES=true`。
- 主/阅读密钥只在创建响应中返回；数据库只保存 Argon2id 哈希。
- 当前密钥查找会遍历现有哈希，适合个人内网低数量实例；规模化前应引入不可逆 key lookup verifier，避免登录随 key 数线性增长。
