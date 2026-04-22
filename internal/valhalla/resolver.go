package valhalla

import (
	"fmt"

	"github.com/swayrider/routerservice/restclients/valhalla"
)

func ResolveHost(c *Config, regionName string) (host string, port int) {
	host, hostOk := c.ValhallaHosts[regionName]
	port, portOk := c.ValhallaPorts[regionName]
	if hostOk && portOk {
		return host, port
	}

	host = fmt.Sprintf("%s%s%s", c.ValhallaPrefix, regionName, c.ValhallaPostfix)
	port = c.ValhallaPort
	return
}

func GetClientForRegions(c *Config, regions []string) (clnt *valhalla.Client) {
	clnt = valhalla.NewClient()
	for _, region := range regions {
		host, port := ResolveHost(c, region)
		clnt.AddRegion(region, host, port)
	}
	return clnt
}
