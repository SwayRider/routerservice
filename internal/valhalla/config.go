package valhalla

import (
	"strings"
	"strconv"
)

type Config struct {
	ValhallaPrefix string
	ValhallaPostfix string
	ValhallaPort int
	ValhallaHosts map[string]string
	ValhallaPorts map[string]int
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
) (err error) {
	c.ValhallaPrefix = valhallaPrefix
	c.ValhallaPostfix = valhallaPostfis
	c.ValhallaPort = valhallaPort

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
		res[parts[0]] = parts[1]
	}
	return res, nil
}

func parsePorts(ports []string) (map[string]int, error) {
	var err error

	res := make(map[string]int)
	for _, port := range ports {
		if port == "" {
			continue
		}
		parts := strings.Split(port, ":")
		res[parts[0]], err = strconv.Atoi(parts[1])
		if err != nil {
			return nil, err
		}
	}
	return res, err
}
