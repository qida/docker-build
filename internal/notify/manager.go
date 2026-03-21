package notify

import (
	"fmt"
	"log"

	"docker-build/internal/config"
)

// 优先级常量
const (
	PriorityLow     = 1
	PriorityDefault = 3
	PriorityHigh    = 4
	PriorityUrgent  = 5
)

// Manager 统一的通知管理器
type Manager struct {
	ntfy     *NtfyNotifier
	dingtalk *DingtalkNotifier
}

// NewManager 创建新的通知管理器
func NewManager(cfg *config.NotifyConfig) *Manager {
	manager := &Manager{}

	if cfg != nil {
		if cfg.Ntfy != nil && cfg.Ntfy.Enabled {
			manager.ntfy = NewNtfyNotifier(cfg.Ntfy)
			log.Printf("[NOTIFY] Ntfy notification enabled: %s", cfg.Ntfy.URL)
		}
		if cfg.Dingtalk != nil && cfg.Dingtalk.Enabled {
			manager.dingtalk = NewDingtalkNotifier(cfg.Dingtalk)
			log.Printf("[NOTIFY] Dingtalk notification enabled")
		}
	}

	return manager
}

// SendBuildStart 发送构建开始通知
func (m *Manager) SendBuildStart(repoName, branch, imageName string) error {
	var errors []error

	if m.ntfy != nil {
		if err := m.ntfy.SendBuildStart(repoName, branch, imageName); err != nil {
			errors = append(errors, err)
		}
	}

	if m.dingtalk != nil {
		if err := m.dingtalk.SendBuildStart(repoName, branch, imageName); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to send some notifications: %v", errors)
	}
	return nil
}

// SendBuildStop 发送构建停止通知
func (m *Manager) SendBuildStop(repoName, branch, imageName string) error {
	var errors []error

	if m.ntfy != nil {
		if err := m.ntfy.SendBuildStop(repoName, branch, imageName); err != nil {
			errors = append(errors, err)
		}
	}

	if m.dingtalk != nil {
		if err := m.dingtalk.SendBuildStop(repoName, branch, imageName); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to send some notifications: %v", errors)
	}
	return nil
}

// SendBuildSuccess 发送构建成功通知
func (m *Manager) SendBuildSuccess(repoName, branch, imageName string) error {
	var errors []error

	if m.ntfy != nil {
		if err := m.ntfy.SendBuildSuccess(repoName, branch, imageName); err != nil {
			errors = append(errors, err)
		}
	}

	if m.dingtalk != nil {
		if err := m.dingtalk.SendBuildSuccess(repoName, branch, imageName); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to send some notifications: %v", errors)
	}
	return nil
}

// SendBuildFailure 发送构建失败通知
func (m *Manager) SendBuildFailure(repoName, branch, errMsg string) error {
	var errors []error

	if m.ntfy != nil {
		if err := m.ntfy.SendBuildFailure(repoName, branch, errMsg); err != nil {
			errors = append(errors, err)
		}
	}

	if m.dingtalk != nil {
		if err := m.dingtalk.SendBuildFailure(repoName, branch, errMsg); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to send some notifications: %v", errors)
	}
	return nil
}
