package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Агент по задачам приложения. Работает ТОЛЬКО в dev-режиме (в production
// исходников нет, поэтому и править нечего). Каждый цикл:
//  1. входит на продакшен-сервер (AGENT_SERVER_URL) под AGENT_USERNAME/AGENT_PASSWORD;
//  2. забирает задачи со статусом «new»;
//  3. выполняет каждую через DeepSeek (function calling): читает/пишет файлы
//     в локальном репозитории, запускает сборки и тесты;
//  4. обновляет статус на сервере: «done» + результат, либо «failed» + ошибка.
//
// Коммиты агент не делает — изменения остаются в рабочем дереве на ревью.
type agent struct {
	serverURL  string
	username   string
	password   string
	apiKey     string
	repoRoot   string
	poll       time.Duration
	client     *http.Client
	sessionTok string // cookie сессии на продакшен-сервере
}

// startAgent запускает агент в фоне, если это dev-режим и настроены учётные
// данные для сервера задач. В остальных случаях — пишет причину отключения.
func startAgent() {
	if os.Getenv("GIN_MODE") == "release" {
		log.Println("AGENT: отключён — в production агент не запускается")
		return
	}
	username := getenvOrEnvFile("AGENT_USERNAME", "")
	password := getenvOrEnvFile("AGENT_PASSWORD", "")
	if username == "" || password == "" {
		log.Println("AGENT: отключён — задайте AGENT_USERNAME и AGENT_PASSWORD (учётка на сервере задач)")
		return
	}
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		log.Println("AGENT: отключён — нет DEEPSEEK_API_KEY")
		return
	}

	pollSec, err := strconv.Atoi(getenvOrEnvFile("AGENT_POLL_INTERVAL", "120"))
	if err != nil || pollSec <= 0 {
		pollSec = 120
	}

	a := &agent{
		serverURL: strings.TrimRight(getenvOrEnvFile("AGENT_SERVER_URL", "https://avakumov.ru"), "/"),
		username:  username,
		password:  password,
		apiKey:    os.Getenv("DEEPSEEK_API_KEY"),
		repoRoot:  detectRepoRoot(),
		poll:      time.Duration(pollSec) * time.Second,
		client:    &http.Client{Timeout: 60 * time.Second},
	}
	log.Printf("AGENT: запущен (dev). Сервер задач: %s, репозиторий: %s, опрос каждые %dс",
		a.serverURL, a.repoRoot, pollSec)
	go a.loop()
}

// detectRepoRoot определяет корень репозитория: по умолчанию это каталог,
// из которого запущен процесс; если это server/ — поднимаемся на уровень выше
// (там лежит frontend/). Можно переопределить через AGENT_REPO_ROOT.
func detectRepoRoot() string {
	if override := os.Getenv("AGENT_REPO_ROOT"); override != "" {
		return override
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	if filepath.Base(cwd) == "server" {
		if _, err := os.Stat(filepath.Join(cwd, "..", "frontend")); err == nil {
			return filepath.Join(cwd, "..")
		}
	}
	return cwd
}

// loop — периодический опрос сервера задач.
func (a *agent) loop() {
	for {
		if err := a.runCycle(); err != nil {
			log.Printf("AGENT: ошибка цикла: %v", err)
		}
		time.Sleep(a.poll)
	}
}

// runCycle — один проход: вход, получение задач, выполнение новых
// и обработка запрошенных деплоев.
func (a *agent) runCycle() error {
	if err := a.ensureSession(); err != nil {
		return err
	}
	tasks, err := a.fetchTasks()
	if err != nil {
		return err
	}

	processed := 0
	for _, t := range tasks {
		if t.Status != taskStatusNew {
			continue
		}
		processed++
		log.Printf("AGENT: беру задачу #%d «%s»", t.ID, t.Title)

		if err := a.updateTask(t.ID, t, taskStatusInProgress, nil, nil, nil, nil); err != nil {
			log.Printf("AGENT: не удалось пометить задачу #%d «в работе»: %v", t.ID, err)
			continue
		}

		result, taskLog, err := a.executeTask(t)
		if err != nil {
			log.Printf("AGENT: задача #%d не выполнена: %v", t.ID, err)
			msg := "Не выполнена: " + err.Error()
			if err2 := a.updateTask(t.ID, t, taskStatusFailed, &msg, &taskLog, nil, nil); err2 != nil {
				log.Printf("AGENT: не удалось пометить задачу #%d «failed»: %v", t.ID, err2)
			}
			continue
		}
		if err := a.updateTask(t.ID, t, taskStatusDone, &result, &taskLog, nil, nil); err != nil {
			log.Printf("AGENT: не удалось пометить задачу #%d «done»: %v", t.ID, err)
		} else {
			log.Printf("AGENT: задача #%d «%s» выполнена", t.ID, t.Title)
		}
	}
	if processed > 0 {
		log.Printf("AGENT: цикл завершён, обработано задач: %d", processed)
	}

	// Запрошенные деплои: коммитим рабочее дерево и запускаем make deploy.
	for _, t := range tasks {
		if !t.DeployRequested {
			continue
		}
		log.Printf("AGENT: задача #%d «%s»: запрошен деплой", t.ID, t.Title)
		a.deployTask(t)
	}
	return nil
}

// deployTask выполняет деплой задачи: коммит изменений, make deploy,
// обновление статуса на сервере.
func (a *agent) deployTask(t AppTask) {
	var logBuf strings.Builder
	logBuf.WriteString("\n=== Деплой ===\n")

	if out, err := a.gitCommit(t); err != nil {
		log.Printf("AGENT: задача #%d: git commit не удался: %v", t.ID, err)
		logBuf.WriteString("git commit: " + err.Error())
	} else {
		logBuf.WriteString(out + "\n")
	}

	deployedAt := time.Now().UTC().Format(time.RFC3339)
	taskLog := logBuf.String()

	deployOut, deployErr := a.runDeploy()
	if deployErr != nil {
		log.Printf("AGENT: задача #%d: деплой не удался: %v", t.ID, deployErr)
		taskLog += "\nДеплой не удался: " + deployErr.Error()
		if len(taskLog) > maxAgentLogChars {
			taskLog = taskLog[:maxAgentLogChars] + "\n...(обрезано)"
		}
		if err2 := a.updateTask(t.ID, t, taskStatusDone, nil, &taskLog, boolPtr(false), nil); err2 != nil {
			log.Printf("AGENT: не удалось обновить задачу #%d после неудачного деплоя: %v", t.ID, err2)
		}
		return
	}

	taskLog += deployOut
	if len(taskLog) > maxAgentLogChars {
		taskLog = taskLog[:maxAgentLogChars] + "\n...(обрезано)"
	}
	if err := a.updateTask(t.ID, t, taskStatusDone, nil, &taskLog, boolPtr(false), &deployedAt); err != nil {
		log.Printf("AGENT: не удалось обновить задачу #%d после деплоя: %v", t.ID, err)
		return
	}
	log.Printf("AGENT: задача #%d «%s» задеплоена (%s)", t.ID, t.Title, deployedAt)
}

// gitCommit коммитит все изменения в рабочем дереве с упоминанием задачи.
// «Нечего коммитить» не считается ошибкой.
func (a *agent) gitCommit(t AppTask) (string, error) {
	if out, err := a.runGit("add", "-A"); err != nil {
		return out, fmt.Errorf("git add: %v", err)
	}

	msg := fmt.Sprintf("avakumov-agent: задача #%d «%s»", t.ID, t.Title)
	out, err := a.runGit("commit", "-m", msg)
	if err != nil {
		if strings.Contains(strings.ToLower(out), "nothing to commit") {
			return "коммитить нечего: изменений в рабочем дереве нет", nil
		}
		return out, fmt.Errorf("git commit: %v", err)
	}
	return "коммит создан: " + msg, nil
}

// runGit выполняет git-команду в корне репозитория.
func (a *agent) runGit(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c := exec.CommandContext(ctx, "git", args...)
	c.Dir = a.repoRoot
	c.Env = append(os.Environ(), "GIT_EDITOR=true")
	out, err := c.CombinedOutput()
	return truncateOutput(out), err
}

// runDeploy запускает make deploy в корне репозитория (таймаут 10 минут).
func (a *agent) runDeploy() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	c := exec.CommandContext(ctx, "make", "deploy")
	c.Dir = a.repoRoot
	out, err := c.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return truncateOutput(out), errors.New("make deploy превысил таймаут 10 минут")
	}
	return truncateOutput(out), err
}

// truncateOutput ограничивает длину вывода команды (для логов и модели).
func truncateOutput(out []byte) string {
	text := string(out)
	if len(text) > 8000 {
		text = text[len(text)-8000:]
		text = "...(обрезано)\n" + text
	}
	return text
}

// boolPtr возвращает указатель на bool.
func boolPtr(b bool) *bool {
	return &b
}

// ensureSession входит на сервер задач, если у агента ещё нет cookie сессии.
func (a *agent) ensureSession() error {
	if a.sessionTok != "" {
		return nil
	}
	body, _ := json.Marshal(map[string]string{
		"username": a.username,
		"password": a.password,
	})
	req, err := http.NewRequest(http.MethodPost, a.serverURL+"/api/login", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("вход на %s: %w", a.serverURL, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("вход на %s: статус %d", a.serverURL, resp.StatusCode)
	}
	for _, c := range resp.Cookies() {
		if c.Name == cookieName {
			a.sessionTok = c.Value
			return nil
		}
	}
	return errors.New("вход на сервер не вернул cookie сессии")
}

// fetchTasks получает список задач с сервера.
func (a *agent) fetchTasks() ([]AppTask, error) {
	req, err := http.NewRequest(http.MethodGet, a.serverURL+"/api/app-tasks", nil)
	if err != nil {
		return nil, err
	}
	a.setCookie(req)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("получение задач: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		a.sessionTok = "" // сессия истекла — перелогинимся в следующем цикле
		return nil, fmt.Errorf("получение задач: сессия истекла (401)")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("получение задач: статус %d: %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		Tasks []AppTask `json:"tasks"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	return parsed.Tasks, nil
}

// updateTask обновляет статус (результат, журнал, флаги деплоя) задачи на сервере.
func (a *agent) updateTask(id int, t AppTask, status string, result, taskLog *string, deployRequested *bool, deployedAt *string) error {
	payload := map[string]any{
		"title":       t.Title,
		"description": t.Description,
		"status":      status,
	}
	if result != nil {
		payload["result"] = *result
	}
	if taskLog != nil {
		payload["log"] = *taskLog
	}
	if deployRequested != nil {
		payload["deploy_requested"] = *deployRequested
	}
	if deployedAt != nil {
		payload["deployed_at"] = *deployedAt
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/api/app-tasks/%d", a.serverURL, id), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	a.setCookie(req)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("обновление задачи #%d: %w", id, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		a.sessionTok = ""
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("обновление задачи #%d: статус %d", id, resp.StatusCode)
	}
	return nil
}

func (a *agent) setCookie(req *http.Request) {
	if a.sessionTok != "" {
		req.Header.Set("Cookie", cookieName+"="+a.sessionTok)
	}
}

// ---------------------------------------------------------------------------
// Выполнение задачи: цикл DeepSeek с инструментами
// ---------------------------------------------------------------------------

const agentSystemPrompt = `Ты — агент-разработчик внутри приложения avakumov (бэкенд Go + Gin в server/, фронтенд React + Vite в frontend/). Тебе дают задачу по модификации приложения.

Репозиторий открыт в корне; пути в инструментах указывай относительно корня.

Инструменты:
- list_dir — список файлов в каталоге;
- read_file — прочитать файл;
- write_file — записать файл (создаёт каталоги при необходимости);
- run_command — выполнить команду в корне репозитория (например: cd server && go build ./..., cd frontend && npm run build, cd server && go test ./...).

Правила:
1. Сначала изучи код, потом меняй. Правки минимальные и точечные.
2. После изменений обязательно собери проект и прогони проверки: go build, go vet, go test в server/; npm run build в frontend/.
3. Если что-то не получается или задача неясна — так и напиши, не выдумывай.
4. НЕ запускай деплой (make deploy, deploy.sh) и не делай git push/коммиты — это отдельный механизм (кнопка «Deploy» у задачи).
5. В конце верни КРАТКИЙ итог на русском (до 500 символов): какие файлы изменены и результат проверок.`

// agentTools — инструменты, доступные модели (формат function calling OpenAI).
var agentTools = []map[string]any{
	{
		"type": "function",
		"function": map[string]any{
			"name":        "list_dir",
			"description": "Список файлов и подкаталогов в указанной папке репозитория.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "Путь относительно корня репозитория (пустая строка — корень)"},
				},
				"required": []string{"path"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]any{
			"name":        "read_file",
			"description": "Прочитать содержимое файла из репозитория.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "Путь относительно корня репозитория"},
				},
				"required": []string{"path"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]any{
			"name":        "write_file",
			"description": "Записать файл в репозиторий (перезаписывает). Создаёт каталоги при необходимости.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":    map[string]any{"type": "string", "description": "Путь относительно корня репозитория"},
					"content": map[string]any{"type": "string", "description": "Полное содержимое файла"},
				},
				"required": []string{"path", "content"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]any{
			"name":        "run_command",
			"description": "Выполнить shell-команду в корне репозитория (сборки, тесты, git diff и т.п.).",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{"type": "string", "description": "Команда для sh -c"},
				},
				"required": []string{"command"},
			},
		},
	},
}

// chatResponse — минимальный ответ chat completions (без stream).
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

// executeTask выполняет задачу через DeepSeek с инструментами.
// Возвращает итоговый ответ агента (краткий итог изменений) и подробный
// журнал выполнения (вызовы инструментов и их результаты).
func (a *agent) executeTask(t AppTask) (result, taskLog string, err error) {
	taskText := t.Title
	if strings.TrimSpace(t.Description) != "" {
		taskText += "\n\n" + t.Description
	}

	var logBuf strings.Builder
	fmt.Fprintf(&logBuf, "Задача #%d «%s»\n", t.ID, t.Title)
	step := 0
	appendLog := func(s string) {
		fmt.Fprintln(&logBuf, s)
		if logBuf.Len() > maxAgentLogChars {
			logBuf.Reset()
			logBuf.WriteString("...(журнал обрезан)\n")
		}
	}

	messages := []map[string]any{
		{"role": "system", "content": agentSystemPrompt},
		{"role": "user", "content": taskText},
	}

	const maxIterations = 40
	for iter := 1; iter <= maxIterations; iter++ {
		resp, err := a.chat(messages)
		if err != nil {
			appendLog("ОШИБКА DeepSeek: " + err.Error())
			return "", logBuf.String(), err
		}
		msg := resp.Choices[0].Message

		// Модель закончила работу — это и есть итог.
		if len(msg.ToolCalls) == 0 {
			out := strings.TrimSpace(msg.Content)
			if out == "" {
				err := errors.New("агент вернул пустой ответ")
				appendLog("ОШИБКА: " + err.Error())
				return "", logBuf.String(), err
			}
			appendLog("\n=== Итог ===\n" + out)
			return out, logBuf.String(), nil
		}

		// Добавляем ответ ассистента с вызовами инструментов в историю.
		toolCalls := make([]map[string]any, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			toolCalls = append(toolCalls, map[string]any{
				"id":   tc.ID,
				"type": "function",
				"function": map[string]any{
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				},
			})
		}
		messages = append(messages, map[string]any{
			"role":       "assistant",
			"content":    msg.Content,
			"tool_calls": toolCalls,
		})

		for _, tc := range msg.ToolCalls {
			step++
			log.Printf("AGENT: задача #%d: %s(%s)", t.ID, tc.Function.Name, tc.Function.Arguments)
			appendLog(fmt.Sprintf("\n[%d] %s(%s)", step, tc.Function.Name, tc.Function.Arguments))

			out, err := a.runTool(tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				appendLog("ОШИБКА ИНСТРУМЕНТА: " + err.Error())
				out = "ОШИБКА ИНСТРУМЕНТА: " + err.Error()
			} else {
				appendLog(out)
			}
			messages = append(messages, map[string]any{
				"role":         "tool",
				"tool_call_id": tc.ID,
				"content":      out,
			})
		}
	}
	err = fmt.Errorf("агент не завершил работу за %d итераций", maxIterations)
	appendLog("ОШИБКА: " + err.Error())
	return "", logBuf.String(), err
}

// maxAgentLogChars — предел размера журнала выполнения, прикрепляемого к задаче.
const maxAgentLogChars = 20000

// chat вызывает DeepSeek chat completions с историей сообщений и инструментами.
func (a *agent) chat(messages []map[string]any) (*chatResponse, error) {
	payload := map[string]any{
		"model":       "deepseek-chat",
		"messages":    messages,
		"tools":       agentTools,
		"temperature": 0.2,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost,
		"https://api.deepseek.com/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("вызов DeepSeek: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DeepSeek вернул статус %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed chatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Choices) == 0 {
		return nil, errors.New("DeepSeek не вернул ответ")
	}
	return &parsed, nil
}

// ---------------------------------------------------------------------------
// Инструменты агента
// ---------------------------------------------------------------------------

// resolvePath приводит путь (относительно корня) к абсолютному и запрещает
// выход за пределы репозитория.
func (a *agent) resolvePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return a.repoRoot, nil
	}
	clean := filepath.Clean(filepath.Join(a.repoRoot, p))
	if clean != a.repoRoot && !strings.HasPrefix(clean, a.repoRoot+string(filepath.Separator)) {
		return "", errors.New("путь вне репозитория запрещён")
	}
	return clean, nil
}

// runTool исполняет вызов инструмента и возвращает результат для модели.
func (a *agent) runTool(name, argsJSON string) (string, error) {
	switch name {
	case "list_dir":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("некорректные аргументы: %v", err)
		}
		dir, err := a.resolvePath(args.Path)
		if err != nil {
			return "", err
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() {
				name += "/"
			}
			fmt.Fprintln(&b, name)
		}
		return b.String(), nil

	case "read_file":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("некорректные аргументы: %v", err)
		}
		p, err := a.resolvePath(args.Path)
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		out := string(data)
		if len(out) > 20000 {
			out = out[:20000] + "\n...(обрезано)"
		}
		return out, nil

	case "write_file":
		var args struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("некорректные аргументы: %v", err)
		}
		p, err := a.resolvePath(args.Path)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(p, []byte(args.Content), 0o644); err != nil {
			return "", err
		}
		return "файл записан: " + args.Path, nil

	case "run_command":
		var args struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("некорректные аргументы: %v", err)
		}
		cmd := strings.TrimSpace(args.Command)
		if cmd == "" {
			return "", errors.New("пустая команда")
		}
		// Запрещённые выражения — чтобы агент не навредил системе.
		// Деплой и пуш выполняются отдельным механизмом (кнопка «Deploy»),
		// а не через инструменты агента.
		for _, banned := range []string{"sudo", "rm -rf /", "mkfs", "shutdown", "reboot", ":(){", "make deploy", "deploy.sh", "git push"} {
			if strings.Contains(cmd, banned) {
				return "", fmt.Errorf("команда содержит запрещённое выражение %q", banned)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		c := exec.CommandContext(ctx, "sh", "-c", cmd)
		c.Dir = a.repoRoot
		out, err := c.CombinedOutput()
		if ctx.Err() == context.DeadlineExceeded {
			return "", errors.New("команда превысила таймаут 2 минуты")
		}
		text := string(out)
		if len(text) > 8000 {
			text = text[len(text)-8000:]
			text = "...(обрезано)\n" + text
		}
		if err != nil {
			return "КОМАНДА ЗАВЕРШИЛАСЬ С ОШИБКОЙ (" + err.Error() + "):\n" + text, nil
		}
		return text, nil
	}
	return "", fmt.Errorf("неизвестный инструмент: %s", name)
}
