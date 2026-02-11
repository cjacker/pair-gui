# Pair-GUI
轻量级的文件互传工具，扫描二维码即可在手机和电脑之间互传文件。

## 功能特性
- 📤 **文件上传**：扫描二维码，将文件上传到电脑
- 📥 **文件下载**：扫描二维码，将文件下载到手机
- ⚡ **跨多平台**：支持Windows、Linux、macOS

## 快速开始

### 1. 下载/编译
#### 直接下载（推荐）
前往 [Releases](https://github.com/cjacker/pair-gui/releases) 下载对应平台的预编译二进制文件。

#### 手动编译
```bash
# 克隆仓库
git clone https://github.com/cjacker/pair-gui.git
cd pair-gui

# Linux/macOS 编译
go build -o pair-gui main.go

# Windows 编译（隐藏控制台窗口）
# 需要安装msys2并安装clang toolchain: pacman -S mingw-w64-clang-x86_64-toolchain
set CGO_ENABLED=1
go build -ldflags -H=windowsgui -o pair-gui.exe main.go
```

> **Windows编译注意**：使用 `-ldflags -H=windowsgui` 参数可隐藏控制台窗口，若需要调试可移除该参数。

### 2. 使用方法

#### 传文件到电脑：

直接点击“启动服务”按钮即可启动“上传服务”并弹出二维码，手机端扫描二维码即可访问“文件上传页面”。上传后的文件将被存储到pair-gui.exe所在目录（您可以将它放在桌面上）。

#### 传文件到手机：

在pair-gui界面点击“选择文件”按钮选择要传到手机的一个或多个文件，文件选择完成后，点击“启动服务”按钮即可启动“下载服务”并弹出二维码，手机端扫描二维码即可访问“文件下载列表”。

## 许可证
[MIT License](LICENSE)
