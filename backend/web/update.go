package web

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cn-maul/Gentry/database"
	"github.com/gin-gonic/gin"
)

type versionResponse struct {
	Version string `json:"version"`
}

type checkUpdateResponse struct {
	HasUpdate     bool   `json:"has_update"`
	LatestVersion string `json:"latest_version"`
	DownloadURL   string `json:"download_url,omitempty"`
	ReleaseNotes  string `json:"release_notes"`
}

type updateStatusResponse struct {
	Status  string `json:"status"` // idle | downloading | done | error
	Message string `json:"message"`
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Body    string        `json:"body"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

const (
	githubRepo      = "cn-maul/Gentry"
	updateUserAgent = "Gentry-Updater"
	apiTimeout      = 10 * time.Second
	downloadTimeout = 30 * time.Minute
	downloadRetries = 4
)

var (
	updateStatus   = "idle"
	updateMessage  = ""
	updateRunning  bool
	updateStatusMu sync.RWMutex
)

func beginUpdate() bool {
	updateStatusMu.Lock()
	if updateRunning {
		updateStatusMu.Unlock()
		return false
	}
	updateRunning = true
	updateStatus = "downloading"
	updateMessage = "正在获取最新版本..."
	updateStatusMu.Unlock()

	log.Printf("[Update] 状态: downloading - %s", updateMessage)
	writeUpdateLog("downloading", updateMessage)
	return true
}

func setUpdateStatus(status, message string) {
	updateStatusMu.Lock()
	updateStatus = status
	updateMessage = message
	if status == "done" || status == "error" {
		updateRunning = false
	}
	updateStatusMu.Unlock()

	log.Printf("[Update] 状态: %s - %s", status, message)
	writeUpdateLog(status, message)
}

func getUpdateStatus() (string, string) {
	updateStatusMu.RLock()
	defer updateStatusMu.RUnlock()
	return updateStatus, updateMessage
}

func writeUpdateLog(status, message string) {
	execPath, err := os.Executable()
	if err != nil {
		return
	}
	logPath := filepath.Join(filepath.Dir(execPath), ".gentry-update.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()
	entry := fmt.Sprintf("[%s] %s: %s\n", time.Now().Format("2006-01-02 15:04:05"), status, message)
	_, _ = file.WriteString(entry)
}

func parseProxyURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("代理地址必须包含协议和主机")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "socks5":
		return parsed, nil
	default:
		return nil, fmt.Errorf("仅支持 http、https 或 socks5 代理")
	}
}

func configuredProxyURL() (*url.URL, error) {
	raw, _ := database.GetSetting("update_proxy")
	return parseProxyURL(raw)
}

func newHTTPClient(timeout time.Duration, proxyURL *url.URL) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	if proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return &http.Client{Transport: transport, Timeout: timeout}
}

func (s *WebServer) getVersion(c *gin.Context) {
	c.JSON(http.StatusOK, NewSuccessResponse(versionResponse{Version: s.version}))
}

func (s *WebServer) getUpdateStatus(c *gin.Context) {
	status, message := getUpdateStatus()
	c.JSON(http.StatusOK, NewSuccessResponse(updateStatusResponse{Status: status, Message: message}))
}

func (s *WebServer) getUpdateProxy(c *gin.Context) {
	proxy, _ := database.GetSetting("update_proxy")
	c.JSON(http.StatusOK, NewSuccessResponse(gin.H{"proxy": proxy}))
}

func (s *WebServer) setUpdateProxy(c *gin.Context) {
	var req struct {
		Proxy string `json:"proxy"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "无效请求"))
		return
	}
	req.Proxy = strings.TrimSpace(req.Proxy)
	if _, err := parseProxyURL(req.Proxy); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, err.Error()))
		return
	}
	if err := database.SetSetting("update_proxy", req.Proxy); err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "保存失败"))
		return
	}
	c.JSON(http.StatusOK, NewSuccessResponse(gin.H{"proxy": req.Proxy}))
}

func (s *WebServer) checkUpdate(c *gin.Context) {
	release, err := fetchLatestRelease()
	if err != nil {
		log.Printf("[Update] 检查更新失败: %v", err)
		c.JSON(http.StatusBadGateway, NewErrorResponse(502, "检查 GitHub Release 失败: "+err.Error()))
		return
	}

	if !isNewerVersion(s.version, release.TagName) {
		c.JSON(http.StatusOK, NewSuccessResponse(checkUpdateResponse{
			HasUpdate:     false,
			LatestVersion: release.TagName,
		}))
		return
	}

	asset, err := platformAsset(release)
	if err != nil {
		c.JSON(http.StatusBadGateway, NewErrorResponse(502, err.Error()))
		return
	}
	c.JSON(http.StatusOK, NewSuccessResponse(checkUpdateResponse{
		HasUpdate:     true,
		LatestVersion: release.TagName,
		DownloadURL:   asset.BrowserDownloadURL,
		ReleaseNotes:  release.Body,
	}))
}

func (s *WebServer) applyUpdate(c *gin.Context) {
	if !beginUpdate() {
		c.JSON(http.StatusConflict, NewErrorResponse(409, "更新正在进行中"))
		return
	}

	c.JSON(http.StatusAccepted, NewSuccessResponse(gin.H{"message": "开始更新"}))
	go func() {
		release, err := fetchLatestRelease()
		if err != nil {
			setUpdateStatus("error", "获取最新版本失败: "+err.Error())
			return
		}
		if !isNewerVersion(s.version, release.TagName) {
			setUpdateStatus("error", "当前已经是最新版本")
			return
		}
		asset, err := platformAsset(release)
		if err != nil {
			setUpdateStatus("error", err.Error())
			return
		}
		setUpdateStatus("downloading", fmt.Sprintf("正在下载 %s...", release.TagName))
		if err := performUpdate(asset.BrowserDownloadURL, asset.Size); err != nil {
			setUpdateStatus("error", "更新失败: "+err.Error())
			return
		}
		setUpdateStatus("done", "更新完成")
	}()
}

func fetchLatestRelease() (*githubRelease, error) {
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest?ts=%d", githubRepo, time.Now().Unix())
	release, directErr := doFetchRelease(endpoint, newHTTPClient(apiTimeout, nil))
	if directErr == nil {
		return release, nil
	}

	proxyURL, proxyConfigErr := configuredProxyURL()
	if proxyConfigErr != nil {
		return nil, fmt.Errorf("直连失败: %w; 代理配置无效: %v", directErr, proxyConfigErr)
	}
	if proxyURL == nil {
		return nil, fmt.Errorf("直连失败: %w; 未配置更新代理", directErr)
	}
	release, proxyErr := doFetchRelease(endpoint, newHTTPClient(apiTimeout, proxyURL))
	if proxyErr == nil {
		return release, nil
	}
	return nil, fmt.Errorf("直连失败: %w; 代理失败: %v", directErr, proxyErr)
}

func doFetchRelease(endpoint string, client *http.Client) (*githubRelease, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", updateUserAgent)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("GitHub API 返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	if strings.TrimSpace(release.TagName) == "" {
		return nil, fmt.Errorf("GitHub API 响应缺少版本号")
	}
	return &release, nil
}

func platformAssetName() string {
	if runtime.GOOS == "windows" {
		return "gentry-windows-amd64.exe"
	}
	return "gentry-linux-amd64"
}

func platformAsset(release *githubRelease) (*githubAsset, error) {
	wanted := platformAssetName()
	for i := range release.Assets {
		if release.Assets[i].Name == wanted && release.Assets[i].BrowserDownloadURL != "" {
			return &release.Assets[i], nil
		}
	}
	return nil, fmt.Errorf("Release %s 中没有 %s", release.TagName, wanted)
}

func isNewerVersion(current, latest string) bool {
	current = strings.TrimPrefix(strings.TrimSpace(current), "v")
	latest = strings.TrimPrefix(strings.TrimSpace(latest), "v")
	if current == "dev" || current == latest {
		return false
	}
	currentParts, currentOK := numericVersion(current)
	latestParts, latestOK := numericVersion(latest)
	if !currentOK || !latestOK {
		return current != latest
	}
	for i := 0; i < len(currentParts) || i < len(latestParts); i++ {
		var currentPart, latestPart int
		if i < len(currentParts) {
			currentPart = currentParts[i]
		}
		if i < len(latestParts) {
			latestPart = latestParts[i]
		}
		if latestPart != currentPart {
			return latestPart > currentPart
		}
	}
	return false
}

func numericVersion(version string) ([]int, bool) {
	version = strings.SplitN(version, "+", 2)[0]
	version = strings.SplitN(version, "-", 2)[0]
	parts := strings.Split(version, ".")
	if len(parts) == 0 {
		return nil, false
	}
	result := make([]int, len(parts))
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return nil, false
		}
		result[i] = value
	}
	return result, true
}

func performUpdate(downloadURL string, expectedSize int64) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取当前程序路径失败: %w", err)
	}
	execPath, err = filepath.Abs(execPath)
	if err != nil {
		return fmt.Errorf("解析绝对路径失败: %w", err)
	}

	newName := ".gentry-new"
	if runtime.GOOS == "windows" {
		newName += ".exe"
	}
	newFile := filepath.Join(filepath.Dir(execPath), newName)
	writeUpdateLog("info", "当前程序路径: "+execPath)
	if err := downloadFile(downloadURL, newFile, expectedSize); err != nil {
		return fmt.Errorf("下载更新文件失败: %w", err)
	}
	writeUpdateLog("info", "下载并校验完成: "+newFile)

	if runtime.GOOS == "windows" {
		return restartWindows(execPath, newFile)
	}
	if err := os.Chmod(newFile, 0755); err != nil {
		_ = os.Remove(newFile)
		return fmt.Errorf("设置执行权限失败: %w", err)
	}
	backupFile := execPath + ".bak"
	if err := os.Rename(execPath, backupFile); err != nil {
		_ = os.Remove(newFile)
		return fmt.Errorf("备份旧程序失败: %w", err)
	}
	if err := os.Rename(newFile, execPath); err != nil {
		_ = os.Rename(backupFile, execPath)
		return fmt.Errorf("替换程序失败，已恢复: %w", err)
	}
	return syscall.Exec(execPath, os.Args, os.Environ())
}

func downloadFile(downloadURL, dest string, expectedSize int64) error {
	var attempts []*http.Client
	proxyURL, proxyErr := configuredProxyURL()
	if proxyErr != nil {
		writeUpdateLog("info", "忽略无效代理配置: "+proxyErr.Error())
	} else if proxyURL != nil {
		attempts = append(attempts, newHTTPClient(downloadTimeout, proxyURL))
	}
	attempts = append(attempts, newHTTPClient(downloadTimeout, nil))

	_ = os.Remove(dest)
	var errors []string
	for i, client := range attempts {
		label := "直连"
		if len(attempts) > 1 && i == 0 {
			label = "代理"
		}
		if err := downloadWithRetries(downloadURL, dest, expectedSize, client, label); err == nil {
			return nil
		} else {
			errors = append(errors, label+": "+err.Error())
			writeUpdateLog("info", label+"下载失败")
		}
	}
	_ = os.Remove(dest)
	return fmt.Errorf("%s", strings.Join(errors, "; "))
}

func downloadWithRetries(downloadURL, dest string, expectedSize int64, client *http.Client, label string) error {
	var lastErr error
	for attempt := 1; attempt <= downloadRetries; attempt++ {
		if err := doDownload(downloadURL, dest, expectedSize, client); err == nil {
			return nil
		} else {
			lastErr = err
			partialSize := fileSize(dest)
			writeUpdateLog("info", fmt.Sprintf("%s下载第 %d 次失败，已保留 %d 字节: %v", label, attempt, partialSize, err))
			if expectedSize > 0 && partialSize > expectedSize {
				_ = os.Remove(dest)
			}
			if attempt < downloadRetries {
				time.Sleep(time.Duration(attempt) * time.Second)
			}
		}
	}
	return lastErr
}

func doDownload(downloadURL, dest string, expectedSize int64, client *http.Client) error {
	resumeAt := fileSize(dest)
	if expectedSize > 0 && resumeAt == expectedSize {
		return validateExecutable(dest)
	}
	if expectedSize > 0 && resumeAt > expectedSize {
		if err := os.Remove(dest); err != nil {
			return err
		}
		resumeAt = 0
	}

	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", updateUserAgent)
	if resumeAt > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeAt))
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if resumeAt > 0 && resp.StatusCode == http.StatusPartialContent {
		expectedPrefix := fmt.Sprintf("bytes %d-", resumeAt)
		if !strings.HasPrefix(resp.Header.Get("Content-Range"), expectedPrefix) {
			return fmt.Errorf("服务器返回了无效的断点范围: %s", resp.Header.Get("Content-Range"))
		}
	}

	flags := os.O_CREATE | os.O_WRONLY
	if resumeAt > 0 && resp.StatusCode == http.StatusPartialContent {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
		resumeAt = 0
	}
	out, err := os.OpenFile(dest, flags, 0600)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(out, resp.Body)
	if copyErr == nil {
		copyErr = out.Sync()
	}
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	totalWritten := resumeAt + written
	if expectedSize > 0 && totalWritten != expectedSize {
		return fmt.Errorf("文件大小不匹配，期望 %d 字节，实际 %d 字节", expectedSize, totalWritten)
	}
	if err := validateExecutable(dest); err != nil {
		return err
	}
	writeUpdateLog("info", fmt.Sprintf("已下载并校验 %d 字节", totalWritten))
	return nil
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func validateExecutable(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	header := make([]byte, 4)
	if _, err := io.ReadFull(file, header); err != nil {
		return fmt.Errorf("更新文件不完整: %w", err)
	}
	if runtime.GOOS == "windows" {
		if header[0] != 'M' || header[1] != 'Z' {
			return fmt.Errorf("下载内容不是 Windows 可执行文件")
		}
		return nil
	}
	if string(header) != "\x7fELF" {
		return fmt.Errorf("下载内容不是 Linux 可执行文件")
	}
	return nil
}
