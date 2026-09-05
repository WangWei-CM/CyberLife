# Cyberlife

当前实现覆盖工程方案中的**第一至六阶段基础版**，并包含第七阶段 Android 薄客户端骨架：身份与人生空间、Now/Past/Future 页面、ACL、权限预设、绝密、评论、里程碑、开源日历、Markdown 编辑器、主题/歌单设置和 Android 轮询骨架。前端视觉层已按《视觉规格书》与三期军令状完成整体重做（三屏各自主题、完整动效体系、悬浮光效、去框线、连续缩放时间轴与日历、自研图片放大层与删除备份）。

## 已实现

- Go/Gin API 骨架与 Vue 3 前端。
- `global.db` 初始化：Admin、Writer、Life、ReaderKey、Session、月库索引。
- 首次启动使用 `CYBERLIFE_ADMIN_PASSWORD` 初始化唯一管理员；密码只保存 Argon2id 哈希。
- Admin 创建 Writer，服务端生成并仅在响应中返回一次主密钥，同时创建对应人生和当月数据库。
- Admin 为指定人生创建、列出、作废 ReaderKey；作废即提升 key version，使既有会话失效。
- Writer/Reader 主密钥登录、Admin 独立密码登录（`/admin`）、HttpOnly Cookie 会话、登出。
- SQLite WAL、外键、busy timeout 与 life 月库初始化。
- Now：北京时间时钟、进行中规划横幅轮播、身体/心情曲线与记录、Markdown 日记（自动保存、图片上传、图片放大、删除备份）、日历即待办。
- Past：可缩放人生时间轴（滚轮连续缩放、拖动平移、点击选日、里程碑勋章、今日标记、左右停靠）、心情/身体趋势曲线、选中日 日记 | 日程、里程碑丰碑、评论。
- Future：规划列表 / 详情（双进度条、进度标记）、自研连续缩放待办日历（7 天 → 100 年，规划横条落位）。
- ACL：阅读者遵循 **绝密 → 创建日锚点 → 权限预设** 的服务端过滤链；无规则的预设默认拒绝。
- 评论与里程碑：评论要求目标先开启评论且经 ACL 可读；里程碑和其标志均独立经过 ACL 过滤。
- 设置页：歌单（三页独立、默认音频、上传）、权限预设、主题三档、顶栏位置、阅读密钥状态；通知中心；绝密模式；音乐真实播放与断点续播。
- `android/` 提供原生 Kotlin WorkManager 轮询客户端骨架；备份与部署要求见 [部署与备份.md](部署与备份.md)。

## 前端结构

```
web/src
├── App.vue               外壳：登录门禁、三屏主题类、顶栏（滑动指示器/停靠/图标）、View Transitions
├── main.ts               注册 v-glow / v-stagger 指令，按顺序引入样式层
├── api/client.ts         接口客户端（含后端 Actor 字段归一化）
├── stores/auth.ts        会话状态
├── lib/motion.ts         动效基建：stagger、跟随光斑、数值插值、rAF 逼近器、主题过渡
├── lib/dates.ts          北京时间与日期映射工具
├── styles/               tokens（三套主题 × 亮暗）、base、motion、glow、shell、components、各页样式
├── components/           AppIcon、MetricLine、LifeAxis、ZoomCalendar、DiaryEditor、ImageLightbox、PlanCarousel …
└── views/                Login / AdminLogin / Now / Past / Future / Settings / Reader / Admin
```

## 启动

开发构建需要 Go 1.24+、Node.js 22+、pnpm；最终 Windows 产物运行不需要安装这些开发依赖。

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

打开 Vite 给出的本地地址（默认 `http://127.0.0.1:5173`）。Web 开发服务器默认把 `/api` 代理至 `127.0.0.1:8080`；如 API 跑在别的端口，在 `web/.env.local` 里写 `CYBERLIFE_API_ORIGIN=http://127.0.0.1:18080` 即可。

### 直接运行最终产物

将 `product/` 整个文件夹复制到另一台 Windows 电脑，双击 `启动 Cyberlife.cmd`。脚本会启动内置 Go 服务、托管已构建的 Web UI，并打开 `http://127.0.0.1:8080/`。首次启动提示管理员密码；数据保存在 `product/runtime-data/`。双击 `停止 Cyberlife.cmd` 可停止服务。

> 注意：`product/bin/cyberlife.exe` 与 `product/web/` 需要在服务端与前端重新构建后更新。当前源码已修复密钥登录在单连接 SQLite 池下的死锁以及历史接口对 NULL 权限预设的扫描错误，旧构建的产物仍带有这两个问题。

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
| GET | `/api/v1/history` | 过去页按日期范围读取（≤ 366 天/次） |
| GET | `/api/v1/plans` · `/api/v1/calendar` | 规划列表 · 待办日历 |

## 安全边界

- 不提交 `runtime-data/`、数据库、上传文件、`.env` 或密钥。
- 生产环境通过 HTTPS 反向代理，并设置 `CYBERLIFE_SECURE_COOKIES=true`。
- 主/阅读密钥只在创建响应中返回；数据库只保存 Argon2id 哈希。
- 当前密钥查找会遍历现有哈希，适合个人内网低数量实例；规模化前应引入不可逆 key lookup verifier，避免登录随 key 数线性增长。
