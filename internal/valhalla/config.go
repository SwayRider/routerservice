package valhalla

import (
	"fmt"
	"strings"
	"strconv"
	"time"
)

type Config struct {
	ValhallaPrefix string
	ValhallaPostfix string
	ValhallaPort int
	ValhallaHosts map[string]string
	ValhallaPorts map[string]int
	RequestTimeout time.Duration
}

func NewConfig() *Config {
	return &Config{}
}

func (c *Config) ParseConfig(
	valhallaPrefix string,
	valhallaPostfis string,
	valhallaPort int,
	valhallaHosts []string,
	valhallaPorts []string,
	valhallaTimeoutSecs int,
) (err error) {
	c.ValhallaPrefix = valhallaPrefix
	c.ValhallaPostfix = valhallaPostfis
	c.ValhallaPort = valhallaPort
	c.RequestTimeout = time.Duration(valhallaTimeoutSecs) * time.Second

	c.ValhallaHosts, err = parseHosts(valhallaHosts)
	if err != nil {
		return
	}

	c.ValhallaPorts, err = parsePorts(valhallaPorts)
	if err != nil {
		return
	}

	return
}

func parseHosts(hosts []string) (map[string]string, error) {
	res := make(map[string]string)
	for _, host := range hosts {
		if host == "" {
			continue
		}
		parts := strings.Split(host, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid host entry %q: want format region:host", host)
		}
		res[parts[0]] = parts[1]
	}
	return res, nil
}

func parsePorts(ports []string) (map[string]int, error) {
	res := make(map[string]int)
	for _, port := range ports {
		if port == "" {
			continue
		}
		parts := strings.Split(port, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid port entry %q: want format region:port", port)
		}
		p, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, err
		}
		res[parts[0]] = p
	}
	return res, nil
}
