package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"docker-build/internal/config"
)

// DingtalkMessage 钉钉消息结构
type DingtalkMessage struct {
	Msgtype  string `json:"msgtype"`
	Markdown struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	} `json:"markdown"`
}

// DingtalkNotifier 钉钉通知发送器
type DingtalkNotifier struct {
	config      *config.DingtalkConfig
	client      *http.Client
	accessToken string
	tokenExpiry time.Time
}

// NewDingtalkNotifier 创建钉钉通知发送器
func NewDingtalkNotifier(config *config.DingtalkConfig) *DingtalkNotifier {
	if config.ClientID == "" || config.ClientSecret == "" {
		log.Printf("[DINGTALK] client_id or client_secret not configured")
		return nil
	}

	notifier := &DingtalkNotifier{
		config: config,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	// 预获取 access token
	if _, err := notifier.getAccessToken(); err != nil {
		log.Printf("[DINGTALK] Failed to get access token: %v", err)
		return nil
	}

	log.Printf("[DINGTALK] Notifier initialized successfully")
	return notifier
}

// getAccessToken 获取钉钉 access token
func (d *DingtalkNotifier) getAccessToken() (string, error) {
	// 如果 token 还在有效期内，直接返回
	if d.accessToken != "" && time.Now().Before(d.tokenExpiry) {
		return d.accessToken, nil
	}

	// 获取新的 access token
	url := "https://api.dingtalk.com/v1.0/oauth2/accessToken"
	reqBody := fmt.Sprintf(`{"appKey":"%s","appSecret":"%s"}`, d.config.ClientID, d.config.ClientSecret)
	req, err := http.NewRequest("POST", url, bytes.NewBufferString(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"accessToken"`
		ExpiresIn   int    `json:"expiresIn"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[ERROR] Body:%s", resp.Body)
		return "", fmt.Errorf("failed to decode token response: %v", err)
	}

	if result.AccessToken == "" {
		return "", fmt.Errorf("failed to get access token from dingtalk")
	}

	d.accessToken = result.AccessToken
	// 提前 10 分钟过期
	d.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn-600) * time.Second)

	log.Printf("[DINGTALK] Got new access token, expires in %d seconds", result.ExpiresIn)
	return d.accessToken, nil
}

// sendMessage 发送钉钉消息
func (d *DingtalkNotifier) sendMessage(title, content string) error {
	if !d.config.Enabled {
		return nil
	}

	if d.client == nil {
		log.Printf("[DINGTALK] client not initialized")
		return nil
	}

	// 获取 access token
	accessToken, err := d.getAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %v", err)
	}

	// 构建消息
	msg := DingtalkMessage{
		Msgtype: "markdown",
	}
	msg.Markdown.Title = title
	msg.Markdown.Text = content

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	// 发送消息到机器人 API
	url := "https://api.dingtalk.com/v1.0/robot/oToMessages/send"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-acs-dingtalk-access-token", accessToken)

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}

	if result.Code != 0 {
		return fmt.Errorf("dingtalk returned error: %d - %s", result.Code, result.Message)
	}

	log.Printf("[DINGTALK] Sent: %s", title)
	return nil
}

// SendBuildStart 发送构建开始通知
func (d *DingtalkNotifier) SendBuildStart(repoName, branch, imageName string) error {
	title := "构建开始通知"
	content := fmt.Sprintf("## 构建开始通知\n\n"+
		"- **仓库**: %s\n"+
		"- **分支**: %s\n"+
		"- **镜像**: %s\n"+
		"- **时间**: %s\n",
		repoName, branch, imageName, time.Now().Format("2006-01-02 15:04:05"))
	return d.sendMessage(title, content)
}

// SendBuildSuccess 发送构建成功通知
func (d *DingtalkNotifier) SendBuildSuccess(repoName, branch, imageName string) error {
	title := "构建成功通知"
	content := fmt.Sprintf("## 🎉 构建成功\n\n"+
		"- **仓库**: %s\n"+
		"- **分支**: %s\n"+
		"- **镜像**: %s\n"+
		"- **时间**: %s\n",
		repoName, branch, imageName, time.Now().Format("2006-01-02 15:04:05"))
	return d.sendMessage(title, content)
}

// SendBuildFailure 发送构建失败通知
func (d *DingtalkNotifier) SendBuildFailure(repoName, branch, errMsg string) error {
	title := "构建失败通知"
	content := fmt.Sprintf("## ❌ 构建失败\n\n"+
		"- **仓库**: %s\n"+
		"- **分支**: %s\n"+
		"- **错误**: %s\n"+
		"- **时间**: %s\n",
		repoName, branch, errMsg, time.Now().Format("2006-01-02 15:04:05"))
	return d.sendMessage(title, content)
}

// SendBuildStop 发送构建停止通知
func (d *DingtalkNotifier) SendBuildStop(repoName, branch, imageName string) error {
	title := "构建停止通知"
	content := fmt.Sprintf("## ⏹️ 构建停止\n\n"+
		"- **仓库**: %s\n"+
		"- **分支**: %s\n"+
		"- **镜像**: %s\n"+
		"- **时间**: %s\n",
		repoName, branch, imageName, time.Now().Format("2006-01-02 15:04:05"))
	return d.sendMessage(title, content)
}
