package pelias

import (
	"fmt"
	"strings"
	"strconv"
)

type Config struct {
	PeliasPrefix string
	PeliasApiPostfix string
	PeliasApiPort int
	PeliasApiHosts map[string]string
	PeliasApiPorts map[string]int
}

func NewConfig() *Config {
	return &Config{}
}

func (c *Config) ParseConfig(
	peliasPrefix string,
	peliasApiPostfix string,
	peliasApiPort int,
	peliasApiHosts []string,
	peliasApiPorts []string,
) (err error) {
	c.PeliasPrefix = peliasPrefix
	c.PeliasApiPostfix = peliasApiPostfix
	c.PeliasApiPort = peliasApiPort

	c.PeliasApiHosts, err = parseHosts(peliasApiHosts)
	if err != nil {
		return
	}

	c.PeliasApiPorts, err = parsePorts(peliasApiPorts)
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
