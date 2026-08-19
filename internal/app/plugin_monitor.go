package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	pluginMonitorInterval = time.Second
	pluginChangeDebounce  = 2 * time.Second
)

type marketStatus struct {
	Installing bool `json:"installing"`
}

type activationInfo struct {
	State string `json:"state"`
}

type installedStatus struct {
	Activation map[string]activationInfo `json:"activation"`
}

// startPluginMonitor 监控 profile 清单。插件安装完成后由桌面原生对话框提示重启，
// 不依赖已导航到 Harness 的 Web 页面仍然拥有 Wails runtime。
func (a *App) startPluginMonitor() {
	a.monitorOnce.Do(func() {
		a.mu.Lock()
		h := a.harness
		a.mu.Unlock()
		if h == nil {
			return
		}
		manifest := filepath.Join(h.DshHome(), "profiles", "web", "package.json")
		previous, _ := os.ReadFile(manifest)
		go a.monitorPluginChanges(h.URL(), manifest, previous)
	})
}

func (a *App) monitorPluginChanges(baseURL, manifest string, previous []byte) {
	var changedAt time.Time
	ticker := time.NewTicker(pluginMonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.monitorDone:
			return
		case now := <-ticker.C:
			current, err := os.ReadFile(manifest)
			if err != nil {
				continue
			}
			if !bytes.Equal(previous, current) {
				previous = append(previous[:0], current...)
				changedAt = now
				continue
			}
			if changedAt.IsZero() || now.Sub(changedAt) < pluginChangeDebounce {
				continue
			}
			a.mu.Lock()
			preinstallBusy := a.preinstallBusy
			a.mu.Unlock()
			if preinstallBusy {
				changedAt = now
				continue
			}

			needsRestart, busy := a.pluginChangeNeedsRestart(baseURL)
			if busy {
				changedAt = now
				continue
			}
			changedAt = time.Time{}
			if needsRestart {
				a.promptPluginRestart()
			}
		}
	}
}

// pluginChangeNeedsRestart 返回 (需要重启, 市场仍在安装)。
// 市场路由尚不可用通常意味着 dshmarket 本身刚完成首次后台安装，也需要重启。
func (a *App) pluginChangeNeedsRestart(baseURL string) (bool, bool) {
	client := &http.Client{Timeout: 2 * time.Second}
	if response, err := client.Get(baseURL + "/dsh-market/status"); err == nil {
		var status marketStatus
		decodeErr := json.NewDecoder(response.Body).Decode(&status)
		_ = response.Body.Close()
		if decodeErr == nil && status.Installing {
			return false, true
		}
	}

	response, err := client.Get(baseURL + "/dsh-market/installed")
	if err != nil {
		return true, false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return true, false
	}
	var installed installedStatus
	if err := json.NewDecoder(response.Body).Decode(&installed); err != nil {
		return true, false
	}
	for _, activation := range installed.Activation {
		if activation.State == "restart" || activation.State == "inert" {
			return true, false
		}
	}
	return false, false
}

func (a *App) promptPluginRestart() {
	choice, err := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
		Type:          wruntime.QuestionDialog,
		Title:         "插件需要重启",
		Message:       "新安装的插件需要重启 Harness 后才能生效。现在重启吗？",
		Buttons:       []string{"稍后", "立即重启"},
		DefaultButton: "立即重启",
		CancelButton:  "稍后",
	})
	if err == nil && choice == "立即重启" {
		a.Restart()
	}
}
