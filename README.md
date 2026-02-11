# Pair-GUI
A lightweight file transfer tool that enables seamless file sharing between mobile phones and computers by scanning a QR code.

## Features
- 📤 **File Upload**: Scan the QR code to upload files to your computer
- 📥 **File Download**: Scan the QR code to download files to your mobile phone
- ⚡ **Cross-Platform**: Supports Windows, Linux, and macOS

## Quick Start

### 1. Download/Compile
#### Direct Download (Recommended)
Go to [Releases](https://github.com/cjacker/pair-gui/releases) to download the precompiled binary for your platform.

#### Manual Compilation
```bash
# Clone the repository
git clone https://github.com/cjacker/pair-gui.git
cd pair-gui

# Compile for Linux/macOS
go build

# Compile for Windows (hide console window)
# Requires msys2 installation and install clang toolchain: pacman -S mingw-w64-clang-x86_64-toolchain
set CGO_ENABLED=1
go build -ldflags -H=windowsgui
```

> **Note for Windows Compilation**: The `-ldflags -H=windowsgui` parameter hides the console window; remove it if debugging is needed.

### 2. Usage

#### Transfer Files to Computer:

Simply click the "Start Service" button to launch the "Upload Service" and display a QR code. Scan the QR code with your mobile phone to access the "File Upload Page". Uploaded files will be saved to the directory where pair-gui.exe is located (you can place it on the desktop for convenience).

#### Transfer Files to Mobile Phone:

Click the "Select Files" button in the pair-gui interface to choose one or more files to transfer to your mobile phone. After selecting the files, click the "Start Service" button to launch the "Download Service" and display a QR code. Scan the QR code with your mobile phone to access the "File Download List".

## License
[MIT License](LICENSE)
