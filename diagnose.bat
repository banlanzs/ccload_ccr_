@echo off
REM ccLoad 连接诊断工具 (Windows)

echo ========================================
echo ccLoad 连接诊断工具
echo ========================================
echo.

echo [1/5] 检查配置文件...
if exist .env (
    echo [OK] .env 文件存在
    findstr /C:"CCLOAD_PASS" .env >nul 2>&1
    if %errorlevel% equ 0 (
        echo [OK] CCLOAD_PASS 已配置
    ) else (
        echo [WARN] CCLOAD_PASS 未配置
    )
    findstr /C:"PORT" .env >nul 2>&1
    if %errorlevel% equ 0 (
        echo [INFO] 自定义端口配置:
        findstr /C:"PORT" .env
    ) else (
        echo [INFO] 使用默认端口: 8080
    )
) else (
    echo [ERROR] .env 文件不存在
    echo [FIX] 运行: copy .env.example .env
)
echo.

echo [2/5] 检查 ccLoad 进程...
tasklist /FI "IMAGENAME eq ccload+ccr.exe" 2>NUL | find /I /N "ccload+ccr.exe">NUL
if %errorlevel% equ 0 (
    echo [OK] ccLoad 进程正在运行
    tasklist /FI "IMAGENAME eq ccload+ccr.exe"
) else (
    echo [ERROR] ccLoad 进程未运行
    echo [FIX] 运行: start.bat 或 ccload+ccr.exe
)
echo.

echo [3/5] 检查端口监听状态...
echo [INFO] 检查常用端口 8080, 8088...
netstat -ano | findstr ":8080 " >nul 2>&1
if %errorlevel% equ 0 (
    echo [OK] 端口 8080 正在监听
    netstat -ano | findstr ":8080 "
) else (
    echo [WARN] 端口 8080 未监听
)

netstat -ano | findstr ":8088 " >nul 2>&1
if %errorlevel% equ 0 (
    echo [OK] 端口 8088 正在监听
    netstat -ano | findstr ":8088 "
) else (
    echo [WARN] 端口 8088 未监听
)
echo.

echo [4/5] 测试本地连接...
echo [INFO] 尝试连接 http://localhost:8080...
curl -s -o nul -w "HTTP Status: %%{http_code}\n" http://localhost:8080/health 2>nul
if %errorlevel% equ 0 (
    echo [OK] 8080 端口可访问
) else (
    echo [ERROR] 8080 端口连接失败
)

echo [INFO] 尝试连接 http://localhost:8088...
curl -s -o nul -w "HTTP Status: %%{http_code}\n" http://localhost:8088/health 2>nul
if %errorlevel% equ 0 (
    echo [OK] 8088 端口可访问
) else (
    echo [ERROR] 8088 端口连接失败
)
echo.

echo [5/5] 检查数据库文件...
if exist data\ccload.db (
    echo [OK] 数据库文件存在: data\ccload.db
    dir data\ccload.db | findstr /C:"ccload.db"
) else (
    echo [WARN] 数据库文件不存在（首次运行会自动创建）
)
echo.

echo ========================================
echo 诊断完成
echo ========================================
echo.
echo 常见问题解决方案:
echo 1. UND_ERR_SOCKET 错误 = 服务未启动
echo    解决: 运行 start.bat 启动服务
echo.
echo 2. 端口冲突
echo    解决: 在 .env 中修改 PORT=其他端口
echo.
echo 3. 连接被拒绝
echo    解决: 检查防火墙设置，允许端口访问
echo.
pause
