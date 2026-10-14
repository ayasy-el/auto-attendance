package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIntervalRangeLegacyAndRandom(t *testing.T) {
	var legacy IntervalRange
	if err := unmarshalYAML(`"2m"`, &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.Min != "2m" || legacy.Max != "2m" {
		t.Fatalf("legacy range = %#v", legacy)
	}
	rangeValue := IntervalRange{Min: "1m", Max: "3m"}
	for i := 0; i < 100; i++ {
		d, err := rangeValue.RandomDuration()
		if err != nil || d < time.Minute || d > 3*time.Minute {
			t.Fatalf("random duration = %s, err=%v", d, err)
		}
	}
}

func TestIntervalRangeRejectsInvertedRange(t *testing.T) {
	if _, err := (IntervalRange{Min: "3m", Max: "1m"}).RandomDuration(); err == nil {
		t.Fatal("expected inverted range error")
	}
}

func TestLoadDefaultsAndEnvironmentExpansion(t *testing.T) {
	t.Setenv("TEST_USER", "student")
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "host: http://host\ncas_host: http://cas\nusername: ${TEST_USER}\npassword: pass\ntahun: 2026\nsemester: 1\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Username != "student" || c.Schedule.InsideClassInterval.Min != "2m" || c.Schedule.OutsideClassInterval.Max != "15m" {
		t.Fatalf("defaults not applied: %#v", c.Schedule)
	}
}
