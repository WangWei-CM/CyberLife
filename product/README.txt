Cyberlife 可迁移 Web 产物

运行：双击“启动 Cyberlife.cmd”。脚本会启动本目录 bin\cyberlife.exe，并打开 http://127.0.0.1:10000/。
首次运行会提示设置管理员密码；之后数据保存在本目录 runtime-data\，请定期备份该目录。

停止：双击“停止 Cyberlife.cmd”。

要求：Windows 10/11；无需安装 Go、Node.js 或 pnpm。

如需修改端口，请编辑启动脚本中的 CYBERLIFE_ADDR。不要把 runtime-data、管理员密码或密钥提交到 Git。
