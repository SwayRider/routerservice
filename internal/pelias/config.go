package pelias

import (
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
