package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Host     string         `yaml:"host"`
	CASHost  string         `yaml:"cas_host"`
	Username string         `yaml:"username"`
	Password string         `yaml:"password"`
	Tahun    int            `yaml:"tahun"`
	Semester int            `yaml:"semester"`
	Timezone string         `yaml:"timezone"`
	Schedule ScheduleConfig `yaml:"schedule"`
	HTTP     HTTPConfig     `yaml:"http"`
}

type ScheduleConfig struct {
	OutsideClassInterval    string `yaml:"outside_class_interval"`
	InsideClassInterval     string `yaml:"inside_class_interval"`
	ActiveStart             string `yaml:"active_start"`
	ActiveEnd               string `yaml:"active_end"`
	RunImmediately          bool   `yaml:"run_immediately"`
	RefreshScheduleInterval string `yaml:"refresh_schedule_interval"`
}

type HTTPConfig struct {
	Timeout   string `yaml:"timeout"`
	UserAgent string `yaml:"user_agent"`
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
	if c.Timezone == "" {
		c.Timezone = "Asia/Jakarta"
	}
	if c.Schedule.OutsideClassInterval == "" {
		c.Schedule.OutsideClassInterval = "15m"
	}
	if c.Schedule.InsideClassInterval == "" {
		c.Schedule.InsideClassInterval = "2m"
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
		c.HTTP.UserAgent = "auto-attendance/1.0"
	}
	for name, value := range map[string]string{"host": c.Host, "cas_host": c.CASHost, "username": c.Username, "password": c.Password} {
		if value == "" {
			return Config{}, fmt.Errorf("config %s wajib diisi", name)
		}
	}
	if c.Tahun == 0 || c.Semester == 0 {
		return Config{}, fmt.Errorf("config tahun dan semester wajib diisi")
	}
	for name, value := range map[string]string{"outside_class_interval": c.Schedule.OutsideClassInterval, "inside_class_interval": c.Schedule.InsideClassInterval, "refresh_schedule_interval": c.Schedule.RefreshScheduleInterval, "http.timeout": c.HTTP.Timeout} {
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
