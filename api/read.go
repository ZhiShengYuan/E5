package api

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"e5autocaller/auth"
	"e5autocaller/config"
	"e5autocaller/notifier"
)

var readAPIList = []string{
	"https://graph.microsoft.com/v1.0/me/",
	"https://graph.microsoft.com/v1.0/users",
	"https://graph.microsoft.com/v1.0/me/people",
	"https://graph.microsoft.com/v1.0/groups",
	"https://graph.microsoft.com/v1.0/me/contacts",
	"https://graph.microsoft.com/v1.0/me/drive/root",
	"https://graph.microsoft.com/v1.0/me/drive/root/children",
	"https://graph.microsoft.com/v1.0/drive/root",
	"https://graph.microsoft.com/v1.0/me/drive",
	"https://graph.microsoft.com/v1.0/me/drive/recent",
	"https://graph.microsoft.com/v1.0/me/drive/sharedWithMe",
	"https://graph.microsoft.com/v1.0/me/calendars",
	"https://graph.microsoft.com/v1.0/me/events",
	"https://graph.microsoft.com/v1.0/sites/root",
	"https://graph.microsoft.com/v1.0/sites/root/sites",
	"https://graph.microsoft.com/v1.0/sites/root/drives",
	"https://graph.microsoft.com/v1.0/sites/root/columns",
	"https://graph.microsoft.com/v1.0/me/onenote/notebooks",
	"https://graph.microsoft.com/v1.0/me/onenote/sections",
	"https://graph.microsoft.com/v1.0/me/onenote/pages",
	"https://graph.microsoft.com/v1.0/me/messages",
	"https://graph.microsoft.com/v1.0/me/mailFolders",
	"https://graph.microsoft.com/v1.0/me/outlook/masterCategories",
	"https://graph.microsoft.com/v1.0/me/mailFolders/Inbox/messages/delta",
	"https://graph.microsoft.com/v1.0/me/mailFolders/inbox/messageRules",
	"https://graph.microsoft.com/v1.0/me/messages?$filter=importance eq 'high'",
	"https://graph.microsoft.com/v1.0/me/messages?$search=\"hello world\"",
	"https://graph.microsoft.com/beta/me/messages?$select=internetMessageHeaders&$top",
}

type ReadRunner struct {
	cfg      *config.Config
	clients  []*auth.TokenManager
	notifier notifier.Notifier
}

func NewReadRunner(cfg *config.Config, n notifier.Notifier) *ReadRunner {
	clients := make([]*auth.TokenManager, len(cfg.Accounts))
	for i := range cfg.Accounts {
		idx := i
		clients[i] = auth.NewTokenManager(
			cfg.Accounts[i].ClientID,
			cfg.Accounts[i].ClientSecret,
			cfg.Accounts[i].RefreshToken,
			cfg.Accounts[i].RedirectURI,
			i+1,
			n,
			func(newToken string) {
				cfg.Accounts[idx].RefreshToken = newToken
			},
		)
	}
	return &ReadRunner{cfg: cfg, clients: clients, notifier: n}
}

func (r *ReadRunner) Run() error {
	if len(r.clients) > 1 {
		fmt.Println("多账户/应用模式下，日志报告里可能会出现一堆***，属于正常情况")
	}
	fmt.Println("如果api数量少于规定值，则是api赋权没有弄好，或者是onedrive还没有初始化成功。前者请重新赋权并获取微软密钥替换，后者请稍等几天")
	fmt.Printf("共 %d 账号/应用，每个账号/应用 %d 轮\n", len(r.clients), r.cfg.ReadConfig.Rounds)

	var apiList []int
	if r.cfg.ReadConfig.ApiRand {
		fixedAPI := []int{0, 1, 5, 6, 20, 21}
		exAPI := []int{2, 3, 4, 7, 8, 9, 10, 22, 23, 24, 25, 26, 27, 13, 14, 15, 16, 17, 18, 19, 11, 12}
		rand.Shuffle(len(exAPI), func(i, j int) { exAPI[i], exAPI[j] = exAPI[j], exAPI[i] })
		fixedAPI = append(fixedAPI, exAPI[:6]...)
		rand.Shuffle(len(fixedAPI), func(i, j int) { fixedAPI[i], fixedAPI[j] = fixedAPI[j], fixedAPI[i] })
		apiList = fixedAPI
	} else {
		apiList = []int{5, 9, 8, 1, 20, 24, 23, 6, 21, 22}
	}

	totalTasks := 0
	var failures []string

	for round := 1; round <= r.cfg.ReadConfig.Rounds; round++ {
		if r.cfg.ReadConfig.RoundsDelay.Enabled {
			delay := rand.Intn(r.cfg.ReadConfig.RoundsDelay.Max-r.cfg.ReadConfig.RoundsDelay.Min+1) + r.cfg.ReadConfig.RoundsDelay.Min
			time.Sleep(time.Duration(delay) * time.Second)
		}

		for appIdx, client := range r.clients {
			appNum := appIdx + 1
			if r.cfg.ReadConfig.AppDelay.Enabled {
				delay := rand.Intn(r.cfg.ReadConfig.AppDelay.Max-r.cfg.ReadConfig.AppDelay.Min+1) + r.cfg.ReadConfig.AppDelay.Min
				time.Sleep(time.Duration(delay) * time.Second)
			}

			fmt.Printf("\n应用/账号 %d 的第%d轮 %s\n", appNum, round, time.Now().Format("Mon Jan 2 15:04:05 2006"))
			if r.cfg.ReadConfig.ApiRand {
				fmt.Println("已开启随机顺序,共十二个api,自己数")
			} else {
				fmt.Println("原版顺序,共十个api,自己数")
			}

			token, err := client.GetAccessToken()
			if err != nil {
				msg := fmt.Sprintf("[E5Autocaller] 账号 %d 获取token失败: %v", appNum, err)
				fmt.Println(msg)
				failures = append(failures, msg)
				continue
			}

			for _, apiIdx := range apiList {
				if apiIdx >= len(readAPIList) {
					continue
				}
				totalTasks++
				url := readAPIList[apiIdx]
				req, _ := http.NewRequest("GET", url, nil)
				req.Header.Set("Authorization", "bearer "+token)
				req.Header.Set("Content-Type", "application/json")

				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					msg := fmt.Sprintf("[E5Autocaller] 账号 %d API %d 请求失败: %v", appNum, apiIdx, err)
					fmt.Println("pass")
					failures = append(failures, msg)
					continue
				}
				resp.Body.Close()

				if resp.StatusCode == 200 {
					fmt.Printf("第%d号api调用成功\n", apiIdx)
				} else {
					msg := fmt.Sprintf("[E5Autocaller] 账号 %d API %d 返回非200状态码: %d", appNum, apiIdx, resp.StatusCode)
					fmt.Println("pass")
					failures = append(failures, msg)
				}

				if r.cfg.ReadConfig.ApiDelay.Enabled {
					delay := rand.Intn(r.cfg.ReadConfig.ApiDelay.Max-r.cfg.ReadConfig.ApiDelay.Min+1) + r.cfg.ReadConfig.ApiDelay.Min
					time.Sleep(time.Duration(delay) * time.Second)
				}
			}
		}
	}

	// 批量通知：失败率 >= 50% 才发
	if totalTasks > 0 && len(failures)*2 >= totalTasks {
		summary := fmt.Sprintf("[E5Autocaller Read] %d/%d tasks failed (\u003e=50%%)\n", len(failures), totalTasks)
		summary += strings.Join(failures, "\n")
		r.notifier.Notify(summary)
	}

	return nil
}

func (r *ReadRunner) Config() *config.Config {
	return r.cfg
}
