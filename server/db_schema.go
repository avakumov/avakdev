package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// pgDumpTimeout — сколько ждём pg_dump, прежде чем прервать.
const pgDumpTimeout = 20 * time.Second

// handleDBSchema отдаёт схему БД (DDL) для админ-раздела «База данных».
// Источник — `pg_dump --schema-only` против DATABASE_URL.
//
// Пароль передаём через переменные окружения libpq (PG*), а не аргументом
// командной строки — чтобы он не светился в списке процессов сервера.
func handleDBSchema(c *gin.Context) {
	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dsn == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "DATABASE_URL не задан"})
		return
	}
	env, err := pgEnvFromDSN(dsn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Некорректный DATABASE_URL"})
		return
	}

	// Путь к pg_dump можно переопределить (например, если его нет в PATH у systemd).
	bin := strings.TrimSpace(os.Getenv("PG_DUMP_BIN"))
	if bin == "" {
		bin = "pg_dump"
	}
	if _, err := exec.LookPath(bin); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "pg_dump не найден. Установите пакет postgresql-client (Debian/Ubuntu) " +
				"или postgresql (Arch/Manjaro), либо задайте путь в PG_DUMP_BIN.",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), pgDumpTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "--schema-only", "--no-owner", "--no-privileges")
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.Output()
	if err != nil {
		msg := "Не удалось получить схему"
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			msg += ": " + strings.TrimSpace(string(ee.Stderr))
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"schema":       string(out),
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// pgEnvFromDSN превращает строку подключения (postgres://…) в набор переменных
// окружения libpq, чтобы не передавать пароль в аргументах pg_dump.
func pgEnvFromDSN(dsn string) ([]string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, err
	}
	env := []string{"PGDATABASE=" + strings.TrimPrefix(u.Path, "/")}
	if h := u.Hostname(); h != "" {
		env = append(env, "PGHOST="+h)
	}
	if p := u.Port(); p != "" {
		env = append(env, "PGPORT="+p)
	}
	if u.User != nil {
		env = append(env, "PGUSER="+u.User.Username())
		if pw, ok := u.User.Password(); ok {
			env = append(env, "PGPASSWORD="+pw)
		}
	}
	if ssl := u.Query().Get("sslmode"); ssl != "" {
		env = append(env, "PGSSLMODE="+ssl)
	}
	return env, nil
}
