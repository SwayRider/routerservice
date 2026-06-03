package valhalla

import (
	"testing"
)

func TestParseHosts(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		m, err := parseHosts(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(m) != 0 {
			t.Errorf("want empty map, got %v", m)
		}
	})

	t.Run("empty string entries skipped", func(t *testing.T) {
		m, err := parseHosts([]string{"", ""})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(m) != 0 {
			t.Errorf("want empty map, got %v", m)
		}
	})

	t.Run("normal entries", func(t *testing.T) {
		m, err := parseHosts([]string{"nl:valhalla-nl", "de:valhalla-de"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m["nl"] != "valhalla-nl" {
			t.Errorf("nl: want valhalla-nl, got %q", m["nl"])
		}
		if m["de"] != "valhalla-de" {
			t.Errorf("de: want valhalla-de, got %q", m["de"])
		}
	})
}

func TestParsePorts(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		m, err := parsePorts(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(m) != 0 {
			t.Errorf("want empty map, got %v", m)
		}
	})

	t.Run("empty string entries skipped", func(t *testing.T) {
		m, err := parsePorts([]string{""})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(m) != 0 {
			t.Errorf("want empty map, got %v", m)
		}
	})

	t.Run("normal entries", func(t *testing.T) {
		m, err := parsePorts([]string{"nl:8001", "de:8002"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m["nl"] != 8001 {
			t.Errorf("nl: want 8001, got %d", m["nl"])
		}
		if m["de"] != 8002 {
			t.Errorf("de: want 8002, got %d", m["de"])
		}
	})

	t.Run("invalid port returns error", func(t *testing.T) {
		_, err := parsePorts([]string{"nl:notanumber"})
		if err == nil {
			t.Error("want error for invalid port, got nil")
		}
	})
}

func TestParseConfig(t *testing.T) {
	c := NewConfig()
	err := c.ParseConfig(
		"valhalla-",
		".example.com",
		8002,
		[]string{"nl:custom-nl-host"},
		[]string{"nl:9001"},
	)
	if err != nil {
		t.Fatalf("ParseConfig error: %v", err)
	}
	if c.ValhallaPrefix != "valhalla-" {
		t.Errorf("prefix: want valhalla-, got %q", c.ValhallaPrefix)
	}
	if c.ValhallaPostfix != ".example.com" {
		t.Errorf("postfix: want .example.com, got %q", c.ValhallaPostfix)
	}
	if c.ValhallaPort != 8002 {
		t.Errorf("port: want 8002, got %d", c.ValhallaPort)
	}
	if c.ValhallaHosts["nl"] != "custom-nl-host" {
		t.Errorf("host override: want custom-nl-host, got %q", c.ValhallaHosts["nl"])
	}
	if c.ValhallaPorts["nl"] != 9001 {
		t.Errorf("port override: want 9001, got %d", c.ValhallaPorts["nl"])
	}
}

func TestResolveHost(t *testing.T) {
	c := &Config{
		ValhallaPrefix:  "valhalla-",
		ValhallaPostfix: ".internal",
		ValhallaPort:    8002,
		ValhallaHosts:   map[string]string{"nl": "custom-nl"},
		ValhallaPorts:   map[string]int{"nl": 9001},
	}

	t.Run("per-region override used when both host and port present", func(t *testing.T) {
		host, port := ResolveHost(c, "nl")
		if host != "custom-nl" || port != 9001 {
			t.Errorf("want custom-nl:9001, got %s:%d", host, port)
		}
	})

	t.Run("fallback to prefix+name+postfix with default port", func(t *testing.T) {
		host, port := ResolveHost(c, "de")
		if host != "valhalla-de.internal" {
			t.Errorf("host: want valhalla-de.internal, got %q", host)
		}
		if port != 8002 {
			t.Errorf("port: want 8002, got %d", port)
		}
	})
}
