# ccLoad 连接问题排查指南

## 问题描述

用户遇到 `API Error: Unable to connect to API (UND_ERR_SOCKET)` 错误，http://localhost:8088 转发请求失败。

## 根本原因

**UND_ERR_SOCKET** 错误表示客户端无法建立 TCP 连接，通常由以下原因导致：

1. **服务未启动**（最常见）
2. **端口配置错误**
3. **防火墙阻止**
4. **端口被占用**

## 诊断步骤

### 1. 快速诊断（推荐）

运行自动诊断工具：

```bash
cd ccLoad
diagnose.bat
```

该工具会自动检查：
- 配置文件状态
- 进程运行状态
- 端口监听状态
- 本地连接测试
- 数据库文件

### 2. 手动诊断

#### 2.1 检查服务状态

```bash
# 检查进程
tasklist | findstr ccload

# 检查端口监听
netstat -ano | findstr :8088
netstat -ano | findstr :8080
```

**预期结果**：
- 应该看到 `ccload+ccr.exe` 进程
- 应该看到端口 8088 或 8080 处于 LISTENING 状态

#### 2.2 检查配置文件

```bash
# 检查 .env 文件是否存在
dir .env

# 查看端口配置
findstr PORT .env
```

**预期结果**：
- `.env` 文件存在
- `PORT=8088` 或使用默认 8080

#### 2.3 测试连接

```bash
# 测试健康检查端点
curl http://localhost:8088/health
curl http://localhost:8080/health
```

**预期结果**：
- 返回 HTTP 200 或 JSON 响应
- 不应该出现 "Connection refused" 或 "Failed to connect"

## 解决方案

### 方案 1：启动服务（最常见）

如果服务未运行：

```bash
cd ccLoad

# 方式 1：使用启动脚本
start.bat

# 方式 2：直接运行
ccload+ccr.exe

# 方式 3：后台运行（Windows）
start /B ccload+ccr.exe
```

**注意**：首次运行前必须配置 `.env` 文件。

### 方案 2：配置端口 8088

如果需要使用 8088 端口：

```bash
# 方式 1：使用预配置文件
copy .env.8088 .env

# 方式 2：手动编辑
# 在 .env 中添加或修改：
# PORT=8088

# 然后启动服务
ccload+ccr.exe
```

### 方案 3：创建配置文件

如果 `.env` 不存在：

```bash
# 从模板创建
copy .env.example .env

# 编辑 .env 文件，至少设置：
# CCLOAD_PASS=your_password
# PORT=8088  # 如果需要 8088 端口

# 启动服务
ccload+ccr.exe
```

### 方案 4：检查端口冲突

如果端口被占用：

```bash
# 查看占用端口的进程
netstat -ano | findstr :8088

# 终止占用进程（替换 PID）
taskkill /PID <进程ID> /F

# 或更改 ccLoad 端口
# 在 .env 中设置其他端口，如 PORT=8089
```

### 方案 5：防火墙配置

如果防火墙阻止：

```bash
# Windows 防火墙添加规则
netsh advfirewall firewall add rule name="ccLoad" dir=in action=allow protocol=TCP localport=8088

# 或通过 Windows 安全中心手动添加
```

## 验证修复

修复后验证服务正常：

```bash
# 1. 检查进程
tasklist | findstr ccload

# 2. 检查端口
netstat -ano | findstr :8088

# 3. 测试连接
curl http://localhost:8088/health

# 4. 查看日志
# 服务启动后会输出日志到控制台
```

**成功标志**：
- 进程运行中
- 端口处于 LISTENING 状态
- curl 返回 200 状态码
- 日志显示 "Server started on :8088"

## 常见错误信息

| 错误 | 原因 | 解决方案 |
|------|------|----------|
| `UND_ERR_SOCKET` | 服务未启动 | 运行 `start.bat` |
| `Connection refused` | 端口未监听 | 检查服务状态和端口配置 |
| `EADDRINUSE` | 端口被占用 | 更改端口或终止占用进程 |
| `CCLOAD_PASS not set` | 缺少密码配置 | 在 `.env` 中设置 `CCLOAD_PASS` |
| `bind: permission denied` | 权限不足 | 使用管理员权限运行 |

## 持久化运行

### Windows 服务方式

使用 NSSM 将 ccLoad 注册为 Windows 服务：

```bash
# 1. 下载 NSSM
# https://nssm.cc/download

# 2. 安装服务
nssm install ccLoad "D:\Documents\ccload+ccr-win64\ccLoad\ccload+ccr.exe"

# 3. 设置工作目录
nssm set ccLoad AppDirectory "D:\Documents\ccload+ccr-win64\ccLoad"

# 4. 启动服务
nssm start ccLoad

# 5. 设置开机自启
nssm set ccLoad Start SERVICE_AUTO_START
```

### 任务计划程序方式

```bash
# 创建开机自启任务
schtasks /create /tn "ccLoad" /tr "D:\Documents\ccload+ccr-win64\ccLoad\ccload+ccr.exe" /sc onstart /ru SYSTEM
```

## 日志查看

服务运行时的日志位置：

- **控制台输出**：直接运行时显示在终端
- **数据库日志**：`data/ccload.db` 中的 `logs` 表
- **Web 界面**：http://localhost:8088/web/logs.html

## 进一步支持

如果问题仍未解决：

1. 运行 `diagnose.bat` 并保存输出
2. 查看服务启动日志
3. 检查 `data/ccload.db` 是否正常创建
4. 提供完整错误信息和环境信息

## 相关文件

- `start.bat` - 快速启动脚本
- `diagnose.bat` - 自动诊断工具
- `.env.8088` - 端口 8088 配置示例
- `.env.example` - 完整配置模板
