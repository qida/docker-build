package notify

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"time"

	"docker-build/internal/config"
)

// NtfyNotifier ntfy 通知发送器
type NtfyNotifier struct {
	config *config.NtfyConfig
	client *http.Client
}

// NewNtfyNotifier 创建 ntfy 通知发送器
func NewNtfyNotifier(config *config.NtfyConfig) *NtfyNotifier {
	return &NtfyNotifier{
		config: config,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// send 发送 ntfy 通知
func (n *NtfyNotifier) send(title, message string, priority int) error {

	if !n.config.Enabled {
		return nil
	}

	if n.config.URL == "" || n.config.Topic == "" {
		log.Printf("[NTFY] url or topic not configured")
		return nil
	}

	url := fmt.Sprintf("%s/%s", n.config.URL, n.config.Topic)

	body := bytes.NewBufferString(message)
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Title", title)
	req.Header.Set("Priority", fmt.Sprintf("%d", priority))

	if n.config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+n.config.Token)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send notification: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ntfy returned status: %d", resp.StatusCode)
	}

	log.Printf("[NTFY] Sent: %s - %s", title, message)
	return nil
}

// SendBuildStart 发送构建开始通知
func (n *NtfyNotifier) SendBuildStart(repoName, branch, imageName string) error {
	title := "🔄 构建开始"
	message := fmt.Sprintf("仓库：%s\n分支：%s\n镜像：%s\n时间：%s",
		repoName, branch, imageName, time.Now().Format("2006-01-02 15:04:05"))
	return n.send(title, message, PriorityDefault)
}

// SendBuildSuccess 发送构建成功通知
func (n *NtfyNotifier) SendBuildSuccess(repoName, branch, imageName string) error {
	title := "✅ 构建成功"
	message := fmt.Sprintf("仓库：%s\n分支：%s\n镜像：%s\n时间：%s",
		repoName, branch, imageName, time.Now().Format("2006-01-02 15:04:05"))
	return n.send(title, message, PriorityDefault)
}

// SendBuildFailure 发送构建失败通知
func (n *NtfyNotifier) SendBuildFailure(repoName, branch, errMsg string) error {
	title := "❌ 构建失败"
	message := fmt.Sprintf("仓库：%s\n分支：%s\n错误：%s\n时间：%s",
		repoName, branch, errMsg, time.Now().Format("2006-01-02 15:04:05"))
	return n.send(title, message, PriorityHigh)
}

// SendBuildStop 发送构建停止通知
func (n *NtfyNotifier) SendBuildStop(repoName, branch, imageName string) error {
	title := "⏹️ 构建停止"
	message := fmt.Sprintf("仓库：%s\n分支：%s\n镜像：%s\n时间：%s",
		repoName, branch, imageName, time.Now().Format("2006-01-02 15:04:05"))
	return n.send(title, message, PriorityDefault)
}
