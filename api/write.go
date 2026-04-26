package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"e5autocaller/auth"
	"e5autocaller/config"
	"e5autocaller/notifier"
)

type WriteRunner struct {
	cfg        *config.Config
	clients    []*auth.TokenManager
	notifier   notifier.Notifier
	totalTasks int
	failures   []string
}

func NewWriteRunner(cfg *config.Config, n notifier.Notifier) *WriteRunner {
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
	return &WriteRunner{cfg: cfg, clients: clients, notifier: n}
}

func (w *WriteRunner) recordFailure(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Println(msg)
	w.failures = append(w.failures, msg)
}

func (w *WriteRunner) apiReq(method string, client *auth.TokenManager, url string, data any) (string, error) {
	w.totalTasks++
	if w.cfg.WriteConfig.ApiDelay.Enabled {
		delay := rand.Intn(w.cfg.WriteConfig.ApiDelay.Max-w.cfg.WriteConfig.ApiDelay.Min+1) + w.cfg.WriteConfig.ApiDelay.Min
		time.Sleep(time.Duration(delay) * time.Second)
	}

	token, err := client.GetAccessToken()
	if err != nil {
		w.recordFailure("[E5Autocaller] Write mode token failure for account %d: %v", client.AppNum(), err)
		fmt.Println("        操作失败")
		return "", err
	}

	var body []byte
	if data != nil {
		body, _ = json.Marshal(data)
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		w.recordFailure("[E5Autocaller] Write mode request build failure: %v", err)
		fmt.Println("        操作失败")
		return "", err
	}
	req.Header.Set("Authorization", "bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.recordFailure("[E5Autocaller] Write mode request failure: %v", err)
		fmt.Println("        操作失败")
		return "", err
	}
	defer resp.Body.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(resp.Body)

	if resp.StatusCode < 300 {
		fmt.Println("        操作成功")
	} else {
		w.recordFailure("[E5Autocaller] Write mode request returned status %d", resp.StatusCode)
		fmt.Println("        操作失败")
	}

	return respBody.String(), nil
}

func (w *WriteRunner) UploadFile(client *auth.TokenManager, appNum int, filename string, data []byte) {
	w.totalTasks++
	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/AutoApi/App%d/%s:/content", appNum, filename)
	req, _ := http.NewRequest("PUT", url, bytes.NewReader(data))
	token, err := client.GetAccessToken()
	if err != nil {
		w.recordFailure("[E5Autocaller] UploadFile token failure for account %d: %v", appNum, err)
		fmt.Println("        上传失败")
		return
	}
	req.Header.Set("Authorization", "bearer "+token)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode >= 300 {
		w.recordFailure("[E5Autocaller] UploadFile failure for account %d: status=%d err=%v", appNum, resp.StatusCode, err)
		fmt.Println("        上传失败")
	} else {
		fmt.Println("        上传成功")
	}
	if resp != nil {
		resp.Body.Close()
	}
}

func (w *WriteRunner) SendEmail(client *auth.TokenManager, appNum int, subject, content string) {
	if w.cfg.Email == "" {
		return
	}
	w.totalTasks++
	url := "https://graph.microsoft.com/v1.0/me/sendMail"
	mailMsg := map[string]any{
		"message": map[string]any{
			"subject": subject,
			"body": map[string]string{
				"contentType": "Text",
				"content":     content,
			},
			"toRecipients": []map[string]any{
				{"emailAddress": map[string]string{"address": w.cfg.Email}},
			},
		},
		"saveToSentItems": "true",
	}
	if _, err := w.apiReq("POST", client, url, mailMsg); err != nil {
		w.recordFailure("[E5Autocaller] SendEmail failure for account %d: %v", appNum, err)
	}
}

func (w *WriteRunner) ExcelWrite(client *auth.TokenManager, appNum int, filename, sheet string) {
	baseURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/AutoApi/App%d/%s:/workbook", appNum, filename)

	fmt.Println("    添加工作表")
	if _, err := w.apiReq("POST", client, baseURL+"/worksheets/add", map[string]string{"name": sheet}); err != nil {
		return
	}

	fmt.Println("    添加表格")
	resp, _ := w.apiReq("POST", client, baseURL+"/worksheets/"+sheet+"/tables/add", map[string]any{
		"address":    "A1:D8",
		"hasHeaders": false,
	})

	var tableResp struct {
		ID string `json:"id"`
	}
	json.Unmarshal([]byte(resp), &tableResp)
	if tableResp.ID == "" {
		w.recordFailure("[E5Autocaller] ExcelWrite failed to get table ID for account %d", appNum)
		return
	}

	fmt.Println("    添加行")
	rows := make([][]int, 2)
	for i := range rows {
		rows[i] = make([]int, 4)
		for j := range rows[i] {
			rows[i][j] = rand.Intn(1200) + 1
		}
	}
	w.apiReq("POST", client, baseURL+"/tables/"+tableResp.ID+"/rows/add", map[string]any{
		"values": rows,
	})
}

func (w *WriteRunner) TaskWrite(client *auth.TokenManager, appNum int, taskname string) {
	url := "https://graph.microsoft.com/v1.0/me/todo/lists"
	fmt.Println("    创建任务列表")
	resp, err := w.apiReq("POST", client, url, map[string]string{"displayName": taskname})
	if err != nil {
		return
	}

	var listResp struct {
		ID string `json:"id"`
	}
	json.Unmarshal([]byte(resp), &listResp)
	if listResp.ID == "" {
		w.recordFailure("[E5Autocaller] TaskWrite failed to get list ID for account %d", appNum)
		return
	}

	fmt.Println("    创建任务")
	taskURL := fmt.Sprintf("%s/%s/tasks", url, listResp.ID)
	resp, err = w.apiReq("POST", client, taskURL, map[string]string{"title": taskname})
	if err != nil {
		return
	}

	var taskResp struct {
		ID string `json:"id"`
	}
	json.Unmarshal([]byte(resp), &taskResp)
	if taskResp.ID == "" {
		w.recordFailure("[E5Autocaller] TaskWrite failed to get task ID for account %d", appNum)
		return
	}

	fmt.Println("    删除任务")
	w.apiReq("DELETE", client, taskURL+"/"+taskResp.ID, nil)

	fmt.Println("    删除任务列表")
	w.apiReq("DELETE", client, url+"/"+listResp.ID, nil)
}

func (w *WriteRunner) TeamWrite(client *auth.TokenManager, appNum int, channelname string) {
	url := "https://graph.microsoft.com/v1.0/me/joinedTeams"
	fmt.Println("    获取team")
	resp, err := w.apiReq("GET", client, url, nil)
	if err != nil {
		return
	}

	var teamsResp struct {
		Value []struct {
			ID string `json:"id"`
		} `json:"value"`
	}
	json.Unmarshal([]byte(resp), &teamsResp)
	if len(teamsResp.Value) == 0 {
		fmt.Println("        没有加入任何team")
		return
	}

	fmt.Println("    创建team频道")
	channelURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/teams/%s/channels", teamsResp.Value[0].ID)
	resp, err = w.apiReq("POST", client, channelURL, map[string]any{
		"displayName":    channelname,
		"description":    "This channel is where we debate all future architecture plans",
		"membershipType": "standard",
	})
	if err != nil {
		return
	}

	var channelResp struct {
		ID string `json:"id"`
	}
	json.Unmarshal([]byte(resp), &channelResp)
	if channelResp.ID == "" {
		w.recordFailure("[E5Autocaller] TeamWrite failed to get channel ID for account %d", appNum)
		return
	}

	fmt.Println("    删除team频道")
	w.apiReq("DELETE", client, channelURL+"/"+channelResp.ID, nil)
}

func (w *WriteRunner) OnenoteWrite(client *auth.TokenManager, appNum int, notename string) {
	url := "https://graph.microsoft.com/v1.0/me/onenote/notebooks"
	fmt.Println("    创建笔记本")
	resp, err := w.apiReq("POST", client, url, map[string]string{"displayName": notename})
	if err != nil {
		return
	}

	var noteResp struct {
		ID string `json:"id"`
	}
	json.Unmarshal([]byte(resp), &noteResp)
	if noteResp.ID == "" {
		w.recordFailure("[E5Autocaller] OnenoteWrite failed to get notebook ID for account %d", appNum)
		return
	}

	fmt.Println("    删除笔记本")
	deleteURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/Notebooks/%s", notename)
	w.apiReq("DELETE", client, deleteURL, nil)
}

func (w *WriteRunner) Run() error {
	// 获取天气
	w.totalTasks++
	weather := ""
	weatherResp, err := http.Get(fmt.Sprintf("http://wttr.in/%s?format=4&m", w.cfg.City))
	if err == nil {
		var buf bytes.Buffer
		buf.ReadFrom(weatherResp.Body)
		weatherResp.Body.Close()
		weather = buf.String()
	} else {
		w.recordFailure("[E5Autocaller] 获取天气失败: %v", err)
	}

	// 发送邮件
	for appIdx, client := range w.clients {
		appNum := appIdx + 1
		fmt.Printf("账号 %d\n", appNum)
		fmt.Println("发送邮件 ( 邮箱单独运行，每次运行只发送一次，防止封号 )")
		if w.cfg.Email != "" {
			w.SendEmail(client, appNum, "weather", weather)
			fmt.Println("")
		}
	}

	// 其他写操作
	for round := 1; round <= w.cfg.WriteConfig.Rounds; round++ {
		if w.cfg.WriteConfig.RoundsDelay.Enabled {
			delay := rand.Intn(w.cfg.WriteConfig.RoundsDelay.Max-w.cfg.WriteConfig.RoundsDelay.Min+1) + w.cfg.WriteConfig.RoundsDelay.Min
			time.Sleep(time.Duration(delay) * time.Second)
		}

		fmt.Printf("第 %d 轮\n\n", round)
		for appIdx, client := range w.clients {
			appNum := appIdx + 1
			if w.cfg.WriteConfig.AppDelay.Enabled {
				delay := rand.Intn(w.cfg.WriteConfig.AppDelay.Max-w.cfg.WriteConfig.AppDelay.Min+1) + w.cfg.WriteConfig.AppDelay.Min
				time.Sleep(time.Duration(delay) * time.Second)
			}

			fmt.Printf("账号 %d\n", appNum)

			// 生成随机xlsx文件
			filename := fmt.Sprintf("QAQ%d.xlsx", rand.Intn(600)+1)
			xlsPath := filepath.Join(os.TempDir(), filename)
			f, err := os.Create(xlsPath)
			if err != nil {
				w.recordFailure("[E5Autocaller] 账号 %d 创建临时文件失败: %v", appNum, err)
				continue
			}
			f.Write([]byte("PK"))
			f.Close()

			data, err := os.ReadFile(xlsPath)
			if err != nil {
				w.recordFailure("[E5Autocaller] 账号 %d 读取临时文件失败: %v", appNum, err)
				continue
			}

			fmt.Println("上传文件 ( 可能会偶尔出现创建上传失败的情况 ) ")
			w.UploadFile(client, appNum, filename, data)

			choosenum := rand.Perm(4)[:2]
			choosetypes := map[int]bool{}
			for _, v := range choosenum {
				choosetypes[v+1] = true
			}

			if w.cfg.WriteConfig.AllStart || choosetypes[1] {
				fmt.Println("excel文件操作")
				w.ExcelWrite(client, appNum, filename, fmt.Sprintf("QVQ%d", rand.Intn(600)+1))
			}
			if w.cfg.WriteConfig.AllStart || choosetypes[2] {
				fmt.Println("team操作")
				w.TeamWrite(client, appNum, fmt.Sprintf("QVQ%d", rand.Intn(600)+1))
			}
			if w.cfg.WriteConfig.AllStart || choosetypes[3] {
				fmt.Println("task操作")
				w.TaskWrite(client, appNum, fmt.Sprintf("QVQ%d", rand.Intn(600)+1))
			}
			if w.cfg.WriteConfig.AllStart || choosetypes[4] {
				fmt.Println("onenote操作")
				w.OnenoteWrite(client, appNum, fmt.Sprintf("QVQ%d", rand.Intn(600)+1))
			}
			fmt.Println("-")
		}
	}

	// 批量通知：失败率 >= 50% 才发
	if w.totalTasks > 0 && len(w.failures)*2 >= w.totalTasks {
		summary := fmt.Sprintf("[E5Autocaller Write] %d/%d tasks failed (\u003e=50%%)\n", len(w.failures), w.totalTasks)
		summary += strings.Join(w.failures, "\n")
		w.notifier.Notify(summary)
	}

	return nil
}

func (w *WriteRunner) Config() *config.Config {
	return w.cfg
}
