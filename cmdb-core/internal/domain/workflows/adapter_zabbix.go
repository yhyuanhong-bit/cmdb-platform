package workflows

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// ZabbixAdapter implements MetricsAdapter for Zabbix.
type ZabbixAdapter struct{}

type zabbixConfig struct {
	APIToken   string   `json:"api_token"`
	Username   string   `json:"username"`
	Password   string   `json:"password"`
	HostGroups []string `json:"host_groups"`
	Items      []string `json:"items"` // item keys like "system.cpu.util", "vm.memory.utilization"
}

type zabbixRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int         `json:"id"`
	Auth    string      `json:"auth,omitempty"`
}

type zabbixRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data"`
	} `json:"error"`
	ID int `json:"id"`
}

func (a *ZabbixAdapter) Fetch(ctx context.Context, endpoint string, config json.RawMessage) ([]MetricPoint, error) {
	var cfg zabbixConfig
	json.Unmarshal(config, &cfg) //nolint:errcheck

	if len(cfg.Items) == 0 {
		return nil, nil
	}

	// Authenticate if needed
	auth := cfg.APIToken
	if auth == "" && cfg.Username != "" {
		var err error
		auth, err = zabbixLogin(ctx, endpoint, cfg.Username, cfg.Password)
		if err != nil {
			return nil, fmt.Errorf("zabbix login: %w", err)
		}
	}
	if auth == "" {
		return nil, fmt.Errorf("zabbix: no api_token or username/password configured")
	}

	// Get hosts (to map hostid → IP)
	hostMap, err := zabbixGetHosts(ctx, endpoint, auth, cfg.HostGroups)
	if err != nil {
		return nil, fmt.Errorf("zabbix get hosts: %w", err)
	}

	// Get item values
	var allPoints []MetricPoint
	for _, itemKey := range cfg.Items {
		points, err := zabbixGetItemValues(ctx, endpoint, auth, itemKey, hostMap)
		if err != nil {
			continue
		}
		allPoints = append(allPoints, points...)
	}

	return allPoints, nil
}

func zabbixRPC(ctx context.Context, endpoint string, method string, params interface{}, auth string) (json.RawMessage, error) {
	reqBody := zabbixRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
		Auth:    auth,
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json-rpc")

	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var rpcResp zabbixRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("zabbix error: %s — %s", rpcResp.Error.Message, rpcResp.Error.Data)
	}
	return rpcResp.Result, nil
}

func zabbixLogin(ctx context.Context, endpoint, username, password string) (string, error) {
	result, err := zabbixRPC(ctx, endpoint, "user.login", map[string]string{
		"username": username,
		"password": password,
	}, "")
	if err != nil {
		return "", err
	}
	var token string
	json.Unmarshal(result, &token) //nolint:errcheck
	return token, nil
}

func zabbixGetHosts(ctx context.Context, endpoint, auth string, hostGroups []string) (map[string]string, error) {
	params := map[string]interface{}{
		"output":           []string{"hostid", "host"},
		"selectInterfaces": []string{"ip"},
	}
	if len(hostGroups) > 0 {
		params["groupids"] = hostGroups
	}

	result, err := zabbixRPC(ctx, endpoint, "host.get", params, auth)
	if err != nil {
		return nil, err
	}

	var hosts []struct {
		HostID     string `json:"hostid"`
		Host       string `json:"host"`
		Interfaces []struct {
			IP string `json:"ip"`
		} `json:"interfaces"`
	}
	json.Unmarshal(result, &hosts) //nolint:errcheck

	hostMap := make(map[string]string)
	for _, h := range hosts {
		ip := ""
		if len(h.Interfaces) > 0 {
			ip = h.Interfaces[0].IP
		}
		hostMap[h.HostID] = ip
	}
	return hostMap, nil
}

func zabbixGetItemValues(ctx context.Context, endpoint, auth, itemKey string, hostMap map[string]string) ([]MetricPoint, error) {
	result, err := zabbixRPC(ctx, endpoint, "item.get", map[string]interface{}{
		"output":    []string{"itemid", "hostid", "name", "lastvalue", "lastclock"},
		"search":    map[string]string{"key_": itemKey},
		"sortfield": "hostid",
	}, auth)
	if err != nil {
		return nil, err
	}

	var items []struct {
		ItemID    string `json:"itemid"`
		HostID    string `json:"hostid"`
		Name      string `json:"name"`
		LastValue string `json:"lastvalue"`
		LastClock string `json:"lastclock"`
	}
	json.Unmarshal(result, &items) //nolint:errcheck

	var points []MetricPoint
	for _, item := range items {
		val, _ := strconv.ParseFloat(item.LastValue, 64)
		ts, _ := strconv.ParseInt(item.LastClock, 10, 64)
		ip := hostMap[item.HostID]

		points = append(points, MetricPoint{
			Name:      itemKey,
			Value:     val,
			Timestamp: time.Unix(ts, 0),
			IP:        ip,
			Labels:    map[string]string{"host_id": item.HostID, "item_name": item.Name},
		})
	}
	return points, nil
}
