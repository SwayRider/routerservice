package valhalla

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	//"io"
	//"os"
	"net/http"

	"github.com/swayrider/routerservice/restclients/valhalla/types"
)

type regionClient struct {
	host string
	port int
}

type Client struct {
	regionClients map[string]regionClient
}

func NewClient(
) *Client {
	return &Client{
		regionClients: make(map[string]regionClient),
	}
}

func (c *Client) AddRegion(
	region string,
	host string,
	port int,
) {
	c.regionClients[region] = regionClient{
		host: host,
		port: port,
	}
}

func (c *Client) HasRegion(
	region string,
) bool {
	_, ok := c.regionClients[region]
	return ok
}

func (c Client) Locate(
	ctx context.Context,
	region string,
	request *types.LocateRequest,
) (*types.LocateResponse, error) {
	if !c.HasRegion(region) {
		return nil, fmt.Errorf("region %s not found", region)
	}

	rc := c.regionClients[region]
	url := fmt.Sprintf(
		"http://%s:%d/locate", rc.host, rc.port)

	buffer := new(bytes.Buffer)
	err := json.NewEncoder(buffer).Encode(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, buffer)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var locateResponse types.LocateResponse
	err = json.NewDecoder(resp.Body).Decode(&locateResponse)
	if err != nil {
		return nil, err
	}

	return &locateResponse, nil
}

func (c Client) Route(
	ctx context.Context,
	region string,
	request *types.RouteRequest,
) (*types.RouteResponse, error) {
	if (!c.HasRegion(region)) {
		return nil, fmt.Errorf("region %s not found", region)
	}

	rc := c.regionClients[region]
	url := fmt.Sprintf(
		"http://%s:%d/route", rc.host, rc.port)

	buffer := new(bytes.Buffer)
	err := json.NewEncoder(buffer).Encode(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, buffer)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	/*os.Remove("./response.json")
	buf, _ := io.ReadAll(resp.Body)
	os.WriteFile("./response.json", buf, 0644)*/

	var routeResponse types.RouteResponse
	err = json.NewDecoder(resp.Body).Decode(&routeResponse)
	if err != nil {
		return nil, err
	}

	return &routeResponse, nil
}
