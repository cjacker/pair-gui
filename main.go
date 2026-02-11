package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/jackpal/gateway"
	"github.com/skip2/go-qrcode"
)

// UploadProgress 上传进度结构体
type UploadProgress struct {
	TotalSize int64
	Uploaded  int64
}

// DownloadFile 下载文件信息结构体
type DownloadFile struct {
	Filename string // 文件名
	AbsPath  string // 绝对路径
	SizeKB   int64  // 文件大小(KB)
}

// 全局变量
var (
	progressMap      = make(map[string]*UploadProgress) // 上传进度映射
	downloadFiles    []DownloadFile                     // 待下载文件列表
	httpServer       *http.Server                       // HTTP服务实例
	mainWindow       fyne.Window                        // 主窗口
	routesRegistered bool                               // 路由是否已注册
	routesMutex      sync.Mutex                         // 路由注册互斥锁
)

func main() {
        log.SetOutput(io.Discard)
	// 1. 初始化：只注册一次路由
	registerRoutesOnce()

	// 创建Fyne应用并强制设置为浅色模式（核心修改）
	myApp := app.New()
	myApp.Settings().SetTheme(theme.LightTheme()) // 切换为LightMode

	// 创建主窗口
	mainWindow = myApp.NewWindow("跨平台文件传输工具")
	mainWindow.Resize(fyne.NewSize(600, 500))

	// 2. 创建UI组件
	// 端口输入框
	portEntry := widget.NewEntry()
	portEntry.SetText("1082")
	portEntry.PlaceHolder = "输入端口号（如1082）"
	portEntry.Validator = func(s string) error {
		_, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("请输入有效的数字端口")
		}
		return nil
	}

	// 已选文件展示标签
	fileLabel := widget.NewLabel("未选择任何文件")
	fileLabel.Wrapping = fyne.TextWrapWord

	// 选择文件按钮
	selectFilesBtn := widget.NewButton("选择需要下载的文件", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()

			// 支持多选文件（Fyne默认单文件，可多次选择添加）
			filePath := reader.URI().Path()
			absPath, err := filepath.Abs(filePath)
			if err != nil {
				dialog.ShowError(fmt.Errorf("获取文件路径失败: %v", err), mainWindow)
				return
			}

			// 验证文件
			fileInfo, err := os.Stat(absPath)
			if err != nil {
				dialog.ShowError(fmt.Errorf("文件不存在: %v", err), mainWindow)
				return
			}
			if fileInfo.IsDir() {
				dialog.ShowError(fmt.Errorf("请选择文件而非目录"), mainWindow)
				return
			}

			// 计算文件大小(KB)
			sizeKB := fileInfo.Size() / 1024
			if fileInfo.Size()%1024 != 0 {
				sizeKB += 1
			}

			// 添加到下载文件列表
			downloadFiles = append(downloadFiles, DownloadFile{
				Filename: filepath.Base(absPath),
				AbsPath:  absPath,
				SizeKB:   sizeKB,
			})

			// 更新文件展示标签
			fileLabel.SetText(fmt.Sprintf("已选择文件：\n%s", getSelectedFilesText()))
		}, mainWindow)
	})

	// 启动服务按钮
	startBtn := widget.NewButton("启动服务", func() {
		// 验证端口
		portStr := portEntry.Text
		port, err := strconv.Atoi(portStr)
		if err != nil {
			dialog.ShowError(fmt.Errorf("端口格式错误: %v", err), mainWindow)
			return
		}

		// 停止已有服务
		if httpServer != nil {
			if err := httpServer.Close(); err != nil {
				log.Printf("停止原有服务失败: %v", err)
			}
			httpServer = nil
		}

		// 获取本机IP
		localIP, err := getLocalIP()
		if err != nil {
			localIP = "localhost"
			log.Printf("获取本机IP失败: %v", err)
		}

		// 仅创建并启动HTTP服务
		addr := fmt.Sprintf(":%d", port)
		httpServer = &http.Server{Addr: addr}

		go func() {
			log.Printf("服务启动成功: http://%s:%d", localIP, port)
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				dialog.ShowError(fmt.Errorf("服务启动失败: %v", err), mainWindow)
			}
		}()

		// 核心修改：动态生成不同页面的URL
		var qrURL string
		if len(downloadFiles) > 0 {
			// 有下载文件：生成下载列表页面URL
			qrURL = fmt.Sprintf("http://%s:%d/download-page", localIP, port)
			log.Printf("生成下载列表页面二维码: %s", qrURL)
		} else {
			// 无下载文件：生成上传页面URL
			qrURL = fmt.Sprintf("http://%s:%d", localIP, port)
			log.Printf("生成上传页面二维码: %s", qrURL)
		}
		// 展示二维码
		showQRCodeDialog(qrURL)
	})

	// 停止服务按钮
	stopBtn := widget.NewButton("停止服务", func() {
		if httpServer != nil {
			if err := httpServer.Close(); err != nil {
				dialog.ShowError(fmt.Errorf("停止服务失败: %v", err), mainWindow)
				return
			}
			httpServer = nil
			dialog.ShowInformation("成功", "服务已停止", mainWindow)
		} else {
			dialog.ShowInformation("提示", "当前无运行中的服务", mainWindow)
		}
	})

	// 3. 组装UI布局
	topContainer := container.NewVBox(
		widget.NewLabel("端口设置："),
		portEntry,
		widget.NewSeparator(),
		widget.NewLabel("文件选择："),
		selectFilesBtn,
		fileLabel,
		widget.NewSeparator(),
	)

	btnContainer := container.NewHBox(
		startBtn,
		stopBtn,
	)

	mainContainer := container.NewBorder(
		topContainer,
		btnContainer,
		nil,
		nil,
		container.NewVBox(), // 中间空白区域
	)

	// 设置主窗口内容
	mainWindow.SetContent(mainContainer)

	// 运行应用
	mainWindow.ShowAndRun()
}

// registerRoutesOnce 确保路由只注册一次
func registerRoutesOnce() {
	routesMutex.Lock()
	defer routesMutex.Unlock()

	if !routesRegistered {
		// 只注册一次路由
		http.HandleFunc("/", indexHandler)                     // 上传页面
		http.HandleFunc("/upload", uploadHandler)              // 上传接口
		http.HandleFunc("/progress", progressHandler)          // 进度查询接口
		http.HandleFunc("/download", downloadHandler)          // 下载接口
		http.HandleFunc("/download-page", downloadListHandler) // 下载列表页面
		routesRegistered = true
		log.Println("路由注册完成（仅执行一次）")
	}
}

// getSelectedFilesText 生成已选文件的展示文本
func getSelectedFilesText() string {
	if len(downloadFiles) == 0 {
		return "未选择任何文件"
	}
	text := ""
	for i, f := range downloadFiles {
		text += fmt.Sprintf("%d. %s (%d KB)\n", i+1, f.Filename, f.SizeKB)
	}
	return text
}

// getLocalIP 获取本机局域网IP
func getLocalIP() (string, error) {
	gwIP, err := gateway.DiscoverGateway()
	if err != nil {
		return "", err
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() {
				continue
			}

			ipv4 := ipnet.IP.To4()
			if ipv4 != nil && ipnet.Contains(gwIP) {
				return ipv4.String(), nil
			}
		}
	}

	return "", fmt.Errorf("未找到有效局域网IP")
}

// 优化：更新二维码对话框的提示信息
func showQRCodeDialog(url string) {
	// 生成二维码图片
	qrBytes, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil {
		dialog.ShowError(fmt.Errorf("生成二维码失败: %v", err), mainWindow)
		return
	}

	// 创建二维码图片资源
	qrResource := fyne.NewStaticResource("qrcode.png", qrBytes)
	qrImage := canvas.NewImageFromResource(qrResource)
	qrImage.SetMinSize(fyne.NewSize(256, 256))
	qrImage.FillMode = canvas.ImageFillContain

	// 动态生成提示文本
	var title, tipText string
	if len(downloadFiles) > 0 {
		title = "文件下载服务已启动"
		tipText = fmt.Sprintf("下载列表地址：%s\n扫码直接进入下载页面", url)
	} else {
		title = "文件上传服务已启动"
		tipText = fmt.Sprintf("上传页面地址：%s\n扫码直接进入上传页面", url)
	}

	// 创建对话框内容
	content := container.NewVBox(
		widget.NewLabel(tipText),
		qrImage,
	)

	// 显示对话框
	dialog.ShowCustom(title, "关闭", content, mainWindow)
}

// -------------------------- HTTP处理器 --------------------------

// indexHandler 上传页面处理器【调整按钮样式：放大字号/尺寸】
func indexHandler(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>文件上传（带进度）</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { max-width: 800px; margin: 2rem auto; padding: 0 1rem; font-family: sans-serif; }
        h1 { text-align: center; margin-bottom: 2rem; font-size: 24px; }
        
        .upload-container { 
            border: 2px dashed #ccc; 
            padding: 3rem 2rem; /* 加大容器内边距 */
            text-align: center; 
            border-radius: 8px; 
            margin-bottom: 2rem; 
        }
        
        #file-input { display: none; }
        
        /* 核心修改：放大按钮尺寸和字号 */
        .select-btn, .upload-btn { 
            padding: 1.2rem 3rem; /* 加大按钮内边距 */
            border: none; 
            border-radius: 8px; /* 加大圆角 */
            color: white; 
            cursor: pointer; 
            margin: 0.8rem; 
            font-size: 18px; /* 放大字号 */
            font-weight: bold; /* 加粗文字 */
            min-width: 200px; /* 最小宽度，保证按钮大小 */
            height: 60px; /* 固定高度 */
        }
        
        .select-btn { background: #4285f4; }
        .upload-btn { background: #0f9d58; }
        
        /* 按钮hover效果 */
        .select-btn:hover, .upload-btn:hover {
            opacity: 0.9;
            transform: scale(1.02); /* 轻微放大，提升交互感 */
        }
        
        .progress-item { margin: 1rem 0; padding: 1rem; border: 1px solid #eee; border-radius: 4px; }
        .progress-bar { height: 20px; background: #eee; border-radius: 10px; overflow: hidden; margin-top: 0.5rem; }
        .progress-fill { height: 100%; background: #4285f4; width: 0%; transition: width 0.3s ease; }
        
        .nav-link { margin-top: 2rem; text-align: center; }
        .nav-link a { 
            color: #4285f4; 
            text-decoration: none; 
            padding: 0.8rem 1.5rem; 
            border: 1px solid #4285f4; 
            border-radius: 4px; 
            font-size: 16px;
        }
        
        .nav-link a:hover { 
            background: #4285f4; 
            color: white; 
        }
    </style>
</head>
<body>
    <h1>多文件上传</h1>
    <div class="upload-container">
        <button class="select-btn" onclick="document.getElementById('file-input').click()">选择文件</button>
        <input type="file" id="file-input" multiple>
        <button class="upload-btn" id="upload-btn" onclick="uploadFiles()" style="display:none;">开始上传</button>
    </div>
    <div id="file-list"></div>
    <div class="nav-link">
        <a href="/download-page">前往文件下载页面</a>
    </div>

    <script>
        let files = [];
        const fileInput = document.getElementById('file-input');
        const uploadBtn = document.getElementById('upload-btn');
        const fileList = document.getElementById('file-list');

        fileInput.addEventListener('change', function(e) {
            files = Array.from(e.target.files);
            if (files.length === 0) return;
            uploadBtn.style.display = 'inline-block';
            fileList.innerHTML = '';
            
            files.forEach((file, index) => {
                const item = document.createElement('div');
                item.className = 'progress-item';
                item.innerHTML = ` + "`" + `
                    <div>${file.name} (${formatSize(file.size)})</div>
                    <div class="progress-bar">
                        <div class="progress-fill" id="progress-${index}"></div>
                    </div>
                    <div id="progress-text-${index}">0%</div>
                ` + "`" + `;
                fileList.appendChild(item);
            });
        });

        function formatSize(bytes) {
            if (bytes < 1024) return bytes + ' B';
            if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB';
            return (bytes / 1048576).toFixed(1) + ' MB';
        }

        function uploadFiles() {
            files.forEach((file, index) => {
                const formData = new FormData();
                formData.append('file', file);
                const uploadId = Math.random().toString(36).substring(2, 15);
                
                const xhr = new XMLHttpRequest();
                xhr.open('POST', '/upload?uploadId=' + uploadId, true);
                xhr.upload.addEventListener('progress', function(e) {
                    if (e.lengthComputable) {
                        const percent = (e.loaded / e.total) * 100;
                        updateProgress(index, percent);
                    }
                });

                xhr.onload = function() {
                    if (xhr.status === 200) {
                        updateProgress(index, 100, '上传完成');
                    } else {
                        updateProgress(index, 0, '上传失败');
                    }
                };

                xhr.onerror = function() {
                    updateProgress(index, 0, '上传失败（网络错误）');
                };

                xhr.send(formData);
            });
            uploadBtn.style.display = 'none';
            fileInput.value = '';
        }

        function updateProgress(index, percent, text = '') {
            const fill = document.getElementById('progress-' + index);
            const textEl = document.getElementById('progress-text-' + index);
            fill.style.width = percent + '%';
            textEl.textContent = text || Math.round(percent) + '%';
            if (text.includes('失败')) fill.style.backgroundColor = '#ea4335';
            if (text.includes('完成')) fill.style.backgroundColor = '#0f9d58';
        }
    </script>
</body>
</html>
	`
	tmpl, err := template.New("upload").Parse(html)
	if err != nil {
		http.Error(w, fmt.Sprintf("解析模板失败: %v", err), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// downloadListHandler 下载列表页面处理器【修复水平对齐问题】
// downloadListHandler 下载列表页面处理器【支持文件名折行】
func downloadListHandler(w http.ResponseWriter, r *http.Request) {
	htmlTemplate := `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>文件下载列表</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { max-width: 800px; margin: 2rem auto; padding: 0 1rem; font-family: sans-serif; }
        h1 { text-align: center; margin-bottom: 2rem; font-size: 24px; }
        
        /* 改用弹性布局容器替代表格，彻底解决列挤压问题 */
        .file-list-container {
            margin-top: 2rem;
            border: 1px solid #eee;
            border-radius: 8px;
            overflow: hidden;
        }
        
        /* 列表头部 */
        .file-list-header {
            display: flex;
            background: #4285f4;
            color: white;
            font-weight: bold;
            font-size: 16px;
        }
        
        /* 列表项 */
        .file-list-item {
            display: flex;
            border-bottom: 1px solid #eee;
            align-items: stretch; /* 改为stretch，让列高度自适应内容 */
        }
        
        /* 最后一项去掉下边框 */
        .file-list-item:last-child {
            border-bottom: none;
        }
        
        /* 列样式 - 核心布局：操作列固定宽度，其余空间分配 + 支持文件名折行 */
        .col-name {
            flex: 1; /* 占剩余所有空间 */
            padding: 1.2rem 1rem; /* 统一内边距 */
            font-size: 16px;
            line-height: 1.6; /* 增大行高，优化折行显示 */
            white-space: normal; /* 允许折行（关键） */
            word-wrap: break-word; /* 长单词/文件名强制折行 */
            word-break: break-all; /* 兼容所有字符的折行（包括中文/英文） */
            align-self: center; /* 垂直居中 */
        }
        
        .col-size {
            width: 100px; /* 固定宽度，足够显示文件大小 */
            padding: 1.2rem 1rem; /* 统一内边距，和其他列保持一致 */
            text-align: center;
            white-space: nowrap; /* 大小数字不折行 */
            font-size: 16px;
            align-self: center; /* 垂直居中 */
        }
        
        .col-op {
            width: 100px; /* 固定宽度，保证按钮不挤压 */
            padding: 1.2rem 1rem; /* 统一内边距 */
            text-align: center;
            align-self: center; /* 垂直居中 */
        }
        
        /* 下载按钮样式 */
        .download-btn {
            display: inline-block;
            background: #4285f4;
            color: white;
            padding: 0.8rem 1.5rem; /* 加大按钮内边距 */
            text-decoration: none;
            border-radius: 6px;
            white-space: nowrap; /* 按钮文字不折行 */
            font-size: 16px; /* 放大按钮文字 */
            width: 80px; /* 按钮固定宽度 */
            text-align: center;
        }
        
        /* 空列表提示 */
        .empty-tip {
            padding: 2rem;
            text-align: center;
            color: #999;
            font-size: 16px;
        }
        
        /* 头部列样式统一 */
        .file-list-header .col-name,
        .file-list-header .col-size,
        .file-list-header .col-op {
            padding: 1.2rem 1rem;
            align-self: center;
        }
        
        .file-list-header .col-name {
            text-align: left; /* 文件名头部左对齐 */
        }
        
        .nav-link { margin-top: 2rem; text-align: center; }
        .nav-link a { 
            color: #4285f4; 
            text-decoration: none; 
            padding: 0.8rem 1.5rem; 
            border: 1px solid #4285f4; 
            border-radius: 4px; 
            font-size: 16px;
        }
        
        .nav-link a:hover { 
            background: #4285f4; 
            color: white; 
        }
    </style>
</head>
<body>
    <h1>文件下载列表</h1>
    
    <div class="file-list-container">
        <!-- 列表头部 -->
        <div class="file-list-header">
            <div class="col-name">文件名</div>
            <div class="col-size">文件大小 (KB)</div>
            <div class="col-op">操作</div>
        </div>
        
        <!-- 列表内容 -->
        {{if eq (len .) 0}}
        <div class="empty-tip">暂无可下载文件</div>
        {{else}}
        {{range .}}
        <div class="file-list-item">
            <div class="col-name">{{.Filename}}</div>
            <div class="col-size">{{.SizeKB}}</div>
            <div class="col-op"><a href="/download?file={{.Filename}}" class="download-btn" download>下载</a></div>
        </div>
        {{end}}
        {{end}}
    </div>
    
    <div class="nav-link">
        <a href="/">前往文件上传页面</a>
    </div>
</body>
</html>
	`
	tmpl, err := template.New("downloadList").Parse(htmlTemplate)
	if err != nil {
		http.Error(w, fmt.Sprintf("解析模板失败: %v", err), http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, downloadFiles); err != nil {
		http.Error(w, fmt.Sprintf("渲染页面失败: %v", err), http.StatusInternalServerError)
		return
	}
}


// uploadHandler 文件上传接口处理器
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持POST方法", http.StatusMethodNotAllowed)
		return
	}

	uploadId := r.URL.Query().Get("uploadId")
	if uploadId == "" {
		http.Error(w, "缺少uploadId参数", http.StatusBadRequest)
		return
	}

	err := r.ParseMultipartForm(100 << 20) // 100MB上传限制
	if err != nil {
		http.Error(w, fmt.Sprintf("解析表单失败: %v", err), http.StatusBadRequest)
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		http.Error(w, fmt.Sprintf("获取文件失败: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 初始化上传进度
	progress := &UploadProgress{
		TotalSize: fileHeader.Size,
		Uploaded:  0,
	}
	progressMap[uploadId] = progress

	// 保存文件到当前目录
	filename := filepath.Base(fileHeader.Filename)
	outFile, err := os.Create(filename)
	if err != nil {
		http.Error(w, fmt.Sprintf("创建文件失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer outFile.Close()

	// 包装Reader以跟踪进度
	progressReader := &ProgressReader{
		Reader:   file,
		Progress: progress,
	}

	// 写入文件
	_, err = io.Copy(outFile, progressReader)
	if err != nil {
		http.Error(w, fmt.Sprintf("保存文件失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 移除进度记录
	delete(progressMap, uploadId)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "文件上传成功: %s", filename)
}

// downloadHandler 文件下载接口处理器
func downloadHandler(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		http.Error(w, "缺少file参数", http.StatusBadRequest)
		return
	}

	// 查找文件
	var targetFile DownloadFile
	found := false
	for _, f := range downloadFiles {
		if f.Filename == filename {
			targetFile = f
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "文件不存在", http.StatusNotFound)
		return
	}

	// 设置下载响应头
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", targetFile.Filename))
	w.Header().Set("Content-Type", "application/octet-stream")

	// 打开文件并写入响应
	file, err := os.Open(targetFile.AbsPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("打开文件失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	_, err = io.Copy(w, file)
	if err != nil {
		http.Error(w, fmt.Sprintf("下载文件失败: %v", err), http.StatusInternalServerError)
		return
	}
}

// progressHandler 上传进度查询接口
func progressHandler(w http.ResponseWriter, r *http.Request) {
	uploadId := r.URL.Query().Get("uploadId")
	if uploadId == "" {
		http.Error(w, "缺少uploadId参数", http.StatusBadRequest)
		return
	}

	progress, exists := progressMap[uploadId]
	if !exists {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"total":0,"uploaded":0}`)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"total":%d,"uploaded":%d}`, progress.TotalSize, atomic.LoadInt64(&progress.Uploaded))
}

// ProgressReader 包装io.Reader以跟踪读取进度
type ProgressReader struct {
	Reader   io.Reader
	Progress *UploadProgress
}

// Read 实现io.Reader接口，更新上传进度
func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	atomic.AddInt64(&pr.Progress.Uploaded, int64(n))
	return
}
