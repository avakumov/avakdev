package app

import (
	"context"
	"math"
	"os"
	"sort"
	"strconv"
	"time"
)

// envReadingSpeed — символов в минуту из переменной окружения
// DAY_READING_SPEED; 0, если переменная не задана или некорректна.
func EnvReadingSpeed() int {
	v := os.Getenv("DAY_READING_SPEED")
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return n
	}
	return 0
}

// ReadingSpeed — скорость чтения пользователя из профиля (users.reading_speed).
// 0 или отсутствие значения = «среднее»: сначала DAY_READING_SPEED, иначе 1500.
func (a *App) ReadingSpeed(username string) int {
	if a.Day != nil && username != "" {
		if v, ok := a.Day.ReadingSpeed(context.Background(), username); ok && v > 0 {
			return v
		}
	}
	if v := EnvReadingSpeed(); v > 0 {
		return v
	}
	return 1500
}

// ReadingMinutes — время повторения заметки по количеству символов.
func ReadingMinutes(content string, speed int) int {
	if speed <= 0 {
		speed = 1500
	}
	chars := len([]rune(content))
	if chars == 0 {
		return 1
	}
	m := int(math.Ceil(float64(chars) / float64(speed)))
	if m < 1 {
		return 1
	}
	return m
}

// TaskMinutes — время задачи: planned_hours * 60, минимум 1 минута.
func TaskMinutes(hours float64) int {
	m := int(math.Round(hours * 60))
	if m < 1 {
		return 1
	}
	return m
}

// HoursLabel форматирует часы: 2 → "2 ч", 2.5 → "2.5 ч".
func HoursLabel(h float64) string {
	s := strconv.FormatFloat(h, 'f', -1, 64)
	return s + " ч"
}

// repeatIntervals — даты повторений конспекта, отсчитанные в днях от даты его
// создания («первого назначения»). Индекс = число нажатий «Я повторил»:
// 0 — свежая заметка доступна сразу; затем повторение через 1, 2, 4, 7, … дней
// после создания.
var repeatIntervals = []int{0, 1, 2, 4, 7, 14, 30, 60}

// NextRepeatDays — сдвиг (в днях от создания) следующего повторения после
// n выполненных повторений.
func NextRepeatDays(n int) int {
	if n < 0 {
		n = 0
	}
	if n >= len(repeatIntervals) {
		return repeatIntervals[len(repeatIntervals)-1]
	}
	return repeatIntervals[n]
}

// ParseNoteTime разбирает время заметки (RFC3339 или ГГГГ-ММ-ДД).
func ParseNoteTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse("2006-01-02", s)
		if err != nil {
			return time.Time{}, err
		}
	}
	return t, nil
}

// NoteDueDate — дата следующего повторения конспекта.
// График отсчитывается от даты создания: свежая заметка (0 повторений)
// доступна сразу, дальше +1, +2, +4, +7, … дней. Когда график пройден
// (7+ повторений), интервал 60 дней отсчитывается от последнего повторения.
func NoteDueDate(created, updated string, repetitions int) (time.Time, error) {
	anchor := created
	interval := 0
	if repetitions >= len(repeatIntervals) {
		// График пройден — держим интервал 60 дней от последнего повторения.
		anchor = updated
		interval = repeatIntervals[len(repeatIntervals)-1]
	} else {
		interval = repeatIntervals[repetitions]
	}
	t, err := ParseNoteTime(anchor)
	if err != nil {
		return time.Time{}, err
	}
	return t.AddDate(0, 0, interval), nil
}

// DueKnowledgeNotes — заметки к повторению на дату day, по возрастанию даты.
// Свежая заметка (0 повторений) доступна сразу после создания, дальше график
// отсчитывается от даты создания. Сравнение идёт по календарной дате, чтобы
// время создания/повторения в течение дня не сдвигало срок.
// Раздел «Знания» общий, поэтому владелец не проверяется.
func (a *App) DueKnowledgeNotes(day string) []Note {
	dayTime, err := time.Parse("2006-01-02", day)
	if err != nil || a.Knowledge == nil {
		return nil
	}
	out := make([]Note, 0)
	for _, n := range a.Knowledge.List() {
		due, err := NoteDueDate(n.Created, n.Updated, n.Repetitions)
		if err != nil {
			continue
		}
		// Свежие конспекты (0 повторений) доступны всегда — их можно добавить
		// в день сразу после создания, независимо от даты создания.
		if n.Repetitions == 0 {
			out = append(out, n)
			continue
		}
		// Приводим срок к началу календарного дня в локальной зоне.
		dueDay := time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, dayTime.Location())
		if dueDay.After(dayTime) {
			continue // срок ещё не наступил
		}
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool {
		di, _ := NoteDueDate(out[i].Created, out[i].Updated, out[i].Repetitions)
		dj, _ := NoteDueDate(out[j].Created, out[j].Updated, out[j].Repetitions)
		// Сравниваем по календарной дате срока.
		diD := time.Date(di.Year(), di.Month(), di.Day(), 0, 0, 0, 0, dayTime.Location())
		djD := time.Date(dj.Year(), dj.Month(), dj.Day(), 0, 0, 0, 0, dayTime.Location())
		if !diD.Equal(djD) {
			return diD.Before(djD)
		}
		return out[i].ID < out[j].ID
	})
	return out
}
