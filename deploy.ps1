# deploy.ps1 - 构建并部署 ccload_modified 到运行目录
# 用法：在项目根目录执行 .\deploy.ps1

$ErrorActionPreference = 'Stop'

$SrcExe  = Join-Path $PSScriptRoot "ccload_modified.exe"
$SrcWeb  = Join-Path $PSScriptRoot "web"
$DstDir  = "D:\Download\ccload_modified"
$DstExe  = Join-Path $DstDir "ccload_modified.exe"
$DstWeb  = Join-Path $DstDir "web"
$ExeName = "ccload_modified"

# ── 1. 编译 ──────────────────────────────────────────────────────────────────
Write-Host "[1/4] 编译中..." -ForegroundColor Cyan
Push-Location $PSScriptRoot
try {
    & go build -o $SrcExe . 2>&1 | Write-Host
    if ($LASTEXITCODE -ne 0) { throw "编译失败，退出码 $LASTEXITCODE" }
} finally {
    Pop-Location
}
Write-Host "      编译完成：$SrcExe" -ForegroundColor Green

# ── 2. 停止正在运行的进程 ─────────────────────────────────────────────────────
Write-Host "[2/4] 检查进程..." -ForegroundColor Cyan
$procs = Get-Process -Name $ExeName -ErrorAction SilentlyContinue
if ($procs) {
    Write-Host "      发现 $($procs.Count) 个运行中的进程，正在停止..." -ForegroundColor Yellow
    $procs | Stop-Process -Force
    # 等待进程完全退出（最多 10 秒）
    $waited = 0
    while ((Get-Process -Name $ExeName -ErrorAction SilentlyContinue) -and $waited -lt 10) {
        Start-Sleep -Milliseconds 500
        $waited += 0.5
    }
    Write-Host "      进程已停止" -ForegroundColor Green
} else {
    Write-Host "      未发现运行中的进程" -ForegroundColor Gray
}

# ── 3. 复制替换文件 ───────────────────────────────────────────────────────────
Write-Host "[3/4] 复制文件..." -ForegroundColor Cyan

# 复制 exe
Copy-Item -Path $SrcExe -Destination $DstExe -Force
Write-Host "      exe  -> $DstExe" -ForegroundColor Gray

# 复制 web/（先删旧目录再复制，确保删除已移除的文件）
if (Test-Path $DstWeb) {
    Remove-Item -Path $DstWeb -Recurse -Force
}
Copy-Item -Path $SrcWeb -Destination $DstWeb -Recurse -Force
Write-Host "      web/ -> $DstWeb" -ForegroundColor Gray

Write-Host "      复制完成" -ForegroundColor Green

# ── 4. 启动新进程 ─────────────────────────────────────────────────────────────
Write-Host "[4/4] 启动服务..." -ForegroundColor Cyan
Start-Process -FilePath $DstExe -WorkingDirectory $DstDir
Write-Host "      已启动：$DstExe" -ForegroundColor Green

Write-Host ""
Write-Host "Deploy done." -ForegroundColor Green
