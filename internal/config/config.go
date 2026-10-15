package config

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Host     string         `yaml:"host"`
	CASHost  string         `yaml:"cas_host"`
	Username string         `yaml:"username"`
	Password string         `yaml:"password"`
	Timezone string         `yaml:"timezone"`
	Schedule ScheduleConfig `yaml:"schedule"`
	HTTP     HTTPConfig     `yaml:"http"`
	Logging  LoggingConfig  `yaml:"logging"`
	Telegram TelegramConfig `yaml:"telegram"`
}

type LoggingConfig struct {
	RetentionDays int `yaml:"retention_days"`
}

type TelegramConfig struct {
	Token  string `yaml:"token"`
	ChatID string `yaml:"chat_id"`
}

type ScheduleConfig struct {
	OutsideClassInterval    IntervalRange `yaml:"outside_class_interval"`
	InsideClassInterval     IntervalRange `yaml:"inside_class_interval"`
	ActiveStart             string        `yaml:"active_start"`
	ActiveEnd               string        `yaml:"active_end"`
	RunImmediately          bool          `yaml:"run_immediately"`
	RefreshScheduleInterval string        `yaml:"refresh_schedule_interval"`
}

// IntervalRange accepts either a duration string (legacy format) or a
// min/max mapping. A new random duration is selected for every scheduler tick.
type IntervalRange struct {
	Min string `yaml:"min"`
	Max string `yaml:"max"`
}

func (r *IntervalRange) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		r.Min, r.Max = value.Value, value.Value
		return nil
	}
	type plain IntervalRange
	var parsed plain
	if err := value.Decode(&parsed); err != nil {
		return err
	}
	*r = IntervalRange(parsed)
	return nil
}

func (r IntervalRange) RandomDuration() (time.Duration, error) {
	min, err := time.ParseDuration(r.Min)
	if err != nil {
		return 0, err
	}
	max, err := time.ParseDuration(r.Max)
	if err != nil {
		return 0, err
	}
	if max < min {
		return 0, fmt.Errorf("max harus lebih besar atau sama dengan min")
	}
	if max == min {
		return min, nil
	}
	return min + time.Duration(rand.Int63n(int64(max-min)+1)), nil
}

type HTTPConfig struct {
	Timeout          string `yaml:"timeout"`
	UserAgent        string `yaml:"user_agent"`
	MaxResponseBody  string `yaml:"max_response_body"`
	MaxResponseBytes int64  `yaml:"-"`
}

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("baca config: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	c.Host, c.CASHost, c.Username, c.Password = os.ExpandEnv(c.Host), os.ExpandEnv(c.CASHost), os.ExpandEnv(c.Username), os.ExpandEnv(c.Password)
	c.Telegram.Token = os.ExpandEnv(c.Telegram.Token)
	c.Telegram.ChatID = os.ExpandEnv(c.Telegram.ChatID)
	if c.Logging.RetentionDays <= 0 {
		c.Logging.RetentionDays = 7
	}
	if c.Timezone == "" {
		c.Timezone = "Asia/Jakarta"
	}
	if c.Schedule.OutsideClassInterval.Min == "" {
		c.Schedule.OutsideClassInterval = IntervalRange{Min: "15m", Max: "15m"}
	}
	if c.Schedule.OutsideClassInterval.Max == "" {
		c.Schedule.OutsideClassInterval.Max = c.Schedule.OutsideClassInterval.Min
	}
	if c.Schedule.InsideClassInterval.Min == "" {
		c.Schedule.InsideClassInterval = IntervalRange{Min: "2m", Max: "2m"}
	}
	if c.Schedule.InsideClassInterval.Max == "" {
		c.Schedule.InsideClassInterval.Max = c.Schedule.InsideClassInterval.Min
	}
	if c.Schedule.ActiveStart == "" {
		c.Schedule.ActiveStart = "08:00"
	}
	if c.Schedule.ActiveEnd == "" {
		c.Schedule.ActiveEnd = "18:00"
	}
	if c.Schedule.RefreshScheduleInterval == "" {
		c.Schedule.RefreshScheduleInterval = "24h"
	}
	if c.HTTP.Timeout == "" {
		c.HTTP.Timeout = "30s"
	}
	if c.HTTP.UserAgent == "" {
		c.HTTP.UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36"
	}
	for name, value := range map[string]string{"host": c.Host, "cas_host": c.CASHost, "username": c.Username, "password": c.Password} {
		if value == "" {
			return Config{}, fmt.Errorf("config %s wajib diisi", name)
		}
	}
	maxBytes, err := parseByteSize(c.HTTP.MaxResponseBody)
	if err != nil {
		return Config{}, fmt.Errorf("http.max_response_body tidak valid: %w", err)
	}
	c.HTTP.MaxResponseBytes = maxBytes
	if c.HTTP.MaxResponseBody == "" {
		c.HTTP.MaxResponseBody = "10MB"
	}
	for name, value := range map[string]IntervalRange{"outside_class_interval": c.Schedule.OutsideClassInterval, "inside_class_interval": c.Schedule.InsideClassInterval} {
		if _, err := value.RandomDuration(); err != nil {
			return Config{}, fmt.Errorf("schedule.%s tidak valid: %w", name, err)
		}
	}
	for name, value := range map[string]string{"refresh_schedule_interval": c.Schedule.RefreshScheduleInterval, "http.timeout": c.HTTP.Timeout} {
		if _, err := time.ParseDuration(value); err != nil {
			return Config{}, fmt.Errorf("%s tidak valid: %w", name, err)
		}
	}
	start, err := time.Parse("15:04", c.Schedule.ActiveStart)
	if err != nil {
		return Config{}, fmt.Errorf("schedule.active_start tidak valid: %w", err)
	}
	end, err := time.Parse("15:04", c.Schedule.ActiveEnd)
	if err != nil {
		return Config{}, fmt.Errorf("schedule.active_end tidak valid: %w", err)
	}
	if !start.Before(end) {
		return Config{}, fmt.Errorf("schedule.active_start harus sebelum schedule.active_end")
	}
	return c, nil
}

func parseByteSize(value string) (int64, error) {
	s := strings.TrimSpace(strings.ToUpper(value))
	if s == "" {
		return 10 * 1024 * 1024, nil
	}

	multiplier := int64(1)
	switch {
	case strings.HasSuffix(s, "GB") || strings.HasSuffix(s, "G"):
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "GB"), "G")
	case strings.HasSuffix(s, "MB") || strings.HasSuffix(s, "M"):
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "MB"), "M")
	case strings.HasSuffix(s, "KB") || strings.HasSuffix(s, "K"):
		multiplier = 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "KB"), "K")
	case strings.HasSuffix(s, "B"):
		multiplier = 1
		s = strings.TrimSuffix(s, "B")
	}

	s = strings.TrimSpace(s)
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("ukuran byte tidak valid: %q", value)
	}
	return n * multiplier, nil
}
