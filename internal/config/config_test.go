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
	content := "host: http://host\ncas_host: http://cas\nusername: ${TEST_USER}\npassword: pass\n"
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
	if c.HTTP.MaxResponseBytes != 10*1024*1024 {
		t.Fatalf("expected default max response bytes 10MB, got %d", c.HTTP.MaxResponseBytes)
	}
}

func TestLoadAllowsConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "host: http://host\ncas_host: http://cas\nusername: user\npassword: pass\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err != nil {
		t.Fatalf("expected config to load successfully, got %v", err)
	}
}

func TestLoadInsecureSkipVerify(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "host: http://host\ncas_host: http://cas\nusername: user\npassword: pass\nhttp:\n  insecure_skip_verify: true\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !c.HTTP.InsecureSkipVerify {
		t.Fatal("expected insecure_skip_verify to be true")
	}
}

func TestParseByteSize(t *testing.T) {
	cases := []struct {
		input    string
		expected int64
		hasErr   bool
	}{
		{"", 10 * 1024 * 1024, false},
		{"10MB", 10 * 1024 * 1024, false},
		{"10M", 10 * 1024 * 1024, false},
		{"10 mb", 10 * 1024 * 1024, false},
		{"500KB", 500 * 1024, false},
		{"500K", 500 * 1024, false},
		{"1GB", 1024 * 1024 * 1024, false},
		{"2048B", 2048, false},
		{"10485760", 10485760, false},
		{"0", 0, true},
		{"-5MB", 0, true},
		{"invalid", 0, true},
	}

	for _, tc := range cases {
		got, err := parseByteSize(tc.input)
		if tc.hasErr && err == nil {
			t.Errorf("parseByteSize(%q) expected error, got nil", tc.input)
		}
		if !tc.hasErr && err != nil {
			t.Errorf("parseByteSize(%q) unexpected error: %v", tc.input, err)
		}
		if !tc.hasErr && got != tc.expected {
			t.Errorf("parseByteSize(%q) = %d, expected %d", tc.input, got, tc.expected)
		}
	}
}
