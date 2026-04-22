package pelias

import (
	"fmt"
)

func ResolveApiHost(c *Config, regionName string) (host string, port int) {
	host, hostOk := c.PeliasApiHosts[regionName]
	port, portOk := c.PeliasApiPorts[regionName]
	if hostOk && portOk {
		return host, port
	}

	host = fmt.Sprintf("%s%s%s", c.PeliasPrefix, regionName, c.PeliasApiPostfix)
	port = c.PeliasApiPort
	return
}
