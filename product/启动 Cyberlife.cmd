@echo off
setlocal
cd /d "%~dp0"
if not defined CYBERLIFE_ADMIN_PASSWORD set /p "CYBERLIFE_ADMIN_PASSWORD=首次运行请输入管理员密码: "
if not defined CYBERLIFE_ADMIN_PASSWORD set "CYBERLIFE_ADMIN_PASSWORD=change-this-password-before-first-use"
set "CYBERLIFE_ADDR=127.0.0.1:8080"
set "CYBERLIFE_DATA_DIR=%~dp0runtime-data"
set "CYBERLIFE_WEB_DIR=%~dp0web"
set "CYBERLIFE_SECURE_COOKIES=false"
start "Cyberlife Server" /min "%~dp0bin\cyberlife.exe"
timeout /t 2 /nobreak >nul
start "" "http://127.0.0.1:8080/"
echo Cyberlife is running at http://127.0.0.1:8080/
echo Close the Cyberlife Server window to stop the service.
