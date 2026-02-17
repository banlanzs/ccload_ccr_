@echo off
REM ccLoad 快速启动脚本 (Windows)

echo [INFO] 检查配置文件...
if not exist .env (
    echo [WARN] .env 文件不存在，从模板创建...
    copy .env.example .env
    echo [INFO] 请编辑 .env 文件设置 CCLOAD_PASS 密码
    echo [INFO] 如需使用 8088 端口，请在 .env 中添加: PORT=8088
    pause
    exit /b 1
)

echo [INFO] 启动 ccLoad 服务...
ccload+ccr.exe
