package pelias

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
		m, err := parseHosts([]string{"nl:pelias-nl", "de:pelias-de"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m["nl"] != "pelias-nl" {
			t.Errorf("nl: want pelias-nl, got %q", m["nl"])
		}
		if m["de"] != "pelias-de" {
			t.Errorf("de: want pelias-de, got %q", m["de"])
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
		m, err := parsePorts([]string{"nl:4100", "de:4200"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m["nl"] != 4100 {
			t.Errorf("nl: want 4100, got %d", m["nl"])
		}
		if m["de"] != 4200 {
			t.Errorf("de: want 4200, got %d", m["de"])
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
		"pelias-",
		".example.com",
		4100,
		[]string{"nl:custom-nl-host"},
		[]string{"nl:5001"},
	)
	if err != nil {
		t.Fatalf("ParseConfig error: %v", err)
	}
	if c.PeliasPrefix != "pelias-" {
		t.Errorf("prefix: want pelias-, got %q", c.PeliasPrefix)
	}
	if c.PeliasApiPostfix != ".example.com" {
		t.Errorf("postfix: want .example.com, got %q", c.PeliasApiPostfix)
	}
	if c.PeliasApiPort != 4100 {
		t.Errorf("port: want 4100, got %d", c.PeliasApiPort)
	}
	if c.PeliasApiHosts["nl"] != "custom-nl-host" {
		t.Errorf("host override: want custom-nl-host, got %q", c.PeliasApiHosts["nl"])
	}
	if c.PeliasApiPorts["nl"] != 5001 {
		t.Errorf("port override: want 5001, got %d", c.PeliasApiPorts["nl"])
	}
}
