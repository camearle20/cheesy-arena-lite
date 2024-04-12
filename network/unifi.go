package network

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/Team254/cheesy-arena-lite/model"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"sync"
)

type UnifiIds struct {
	Red1NetworkId  string
	Red2NetworkId  string
	Red3NetworkId  string
	Blue1NetworkId string
	Blue2NetworkId string
	Blue3NetworkId string
	Red1WifiId     string
	Red2WifiId     string
	Red3WifiId     string
	Blue1WifiId    string
	Blue2WifiId    string
	Blue3WifiId    string
}

type UnifiClient struct {
	unifiAddress  string
	unifiUsername string
	unifiPassword string
	networkIds    UnifiIds
	client        *http.Client
	mutex         sync.Mutex
}

type UnifiConfiguredSsids struct {
	red1Ssid  string
	red2Ssid  string
	red3Ssid  string
	blue1Ssid string
	blue2Ssid string
	blue3Ssid string
}

type UnifiWlanconfigData struct {
	Name string `json:"name"`
	Id   string `json:"_id"`
}

type UnifiWlanconfigResponse struct {
	Data []UnifiWlanconfigData `json:"data"`
}

func NewUnifiClient(unifiAddress, unifiUsername, unifiPassword string, networkIds UnifiIds) *UnifiClient {
	jar, _ := cookiejar.New(nil)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Jar:       jar,
		Transport: tr,
	}
	return &UnifiClient{
		unifiAddress:  unifiAddress,
		unifiUsername: unifiUsername,
		unifiPassword: unifiPassword,
		networkIds:    networkIds,
		client:        client,
	}
}

func (unifi *UnifiClient) unifiLogin() error {
	unifi.mutex.Lock()
	defer unifi.mutex.Unlock()

	data := map[string]interface{}{
		"username": unifi.unifiUsername,
		"password": unifi.unifiPassword,
	}
	jsonData, _ := json.Marshal(data)

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("https://%s/api/login", unifi.unifiAddress),
		bytes.NewBufferString(string(jsonData)))

	if err != nil {
		return fmt.Errorf("creating login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := unifi.client.Do(req)

	if err != nil {
		return fmt.Errorf("executing login request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected login status code %d", resp.StatusCode)
	}

	fmt.Printf("Logged into unifi controller\n")
	return nil
}

func (unifi *UnifiClient) unifiConfigureNetworks(teams [6]*model.Team) error {
	unifi.mutex.Lock()
	defer unifi.mutex.Unlock()

	configureNetwork := func(octet1, octet2 int, networkId string) error {
		data := map[string]interface{}{
			"dhcpd_start": fmt.Sprintf("10.%d.%d.20", octet1, octet2),
			"dhcpd_stop":  fmt.Sprintf("10.%d.%d.199", octet1, octet2),
			"ip_subnet":   fmt.Sprintf("10.%d.%d.4/24", octet1, octet2),
		}
		jsonData, _ := json.Marshal(data)

		req, err := http.NewRequest(
			"PUT",
			fmt.Sprintf("https://%s/api/s/default/rest/networkconf/%s", unifi.unifiAddress, networkId),
			bytes.NewBufferString(string(jsonData)))

		if err != nil {
			return fmt.Errorf("creating network config request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := unifi.client.Do(req)

		if err != nil {
			return fmt.Errorf("executing network config request: %w", err)
		}

		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("reading network config response body: %w", err)
			}
			return fmt.Errorf("unexpected networkconf status code %d: %s", resp.StatusCode, body)
		}

		log.Printf("Configured network for 10.%d.%d.0\n", octet1, octet2)
		return nil
	}

	networkIds := [6]string{
		unifi.networkIds.Red1NetworkId,
		unifi.networkIds.Red2NetworkId,
		unifi.networkIds.Red3NetworkId,
		unifi.networkIds.Blue1NetworkId,
		unifi.networkIds.Blue2NetworkId,
		unifi.networkIds.Blue3NetworkId,
	}

	for i := 0; i < 6; i++ {
		if teams[i] == nil {
			continue
		}

		// Clear network config
		err := configureNetwork(0, 101+i, networkIds[i])
		if err != nil {
			return err
		}

		// Configure network
		err = configureNetwork(teams[i].Id/100, teams[i].Id%100, networkIds[i])
		if err != nil {
			return err
		}
	}

	return nil
}

func (unifi *UnifiClient) unifiConfigureWifi(teams [6]*model.Team) error {
	unifi.mutex.Lock()
	defer unifi.mutex.Unlock()

	configureWlan := func(team *model.Team, networkId, wifiId string) error {
		data := map[string]interface{}{
			"networkconf_id": networkId,
			"name":           fmt.Sprintf("%d", team.Id),
			"x_passphrase":   "bluegold", //TODO
		}
		jsonData, _ := json.Marshal(data)

		req, err := http.NewRequest(
			"PUT",
			fmt.Sprintf("https://%s/api/s/default/rest/wlanconf/%s", unifi.unifiAddress, wifiId),
			bytes.NewBufferString(string(jsonData)))

		log.Printf("%s\n", string(jsonData))

		if err != nil {
			return fmt.Errorf("creating wlan config request for team %d: %w", team.Id, err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := unifi.client.Do(req)

		if err != nil {
			return fmt.Errorf("executing wlan config request for team %d request: %w", team.Id, err)
		}

		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return fmt.Errorf("unexpected status code %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)

		fmt.Printf("%s\n", body)

		fmt.Printf("Configured wlan for team %d\n", team.Id)
		return nil
	}

	networkIds := [6]string{
		unifi.networkIds.Red1NetworkId,
		unifi.networkIds.Red2NetworkId,
		unifi.networkIds.Red3NetworkId,
		unifi.networkIds.Blue1NetworkId,
		unifi.networkIds.Blue2NetworkId,
		unifi.networkIds.Blue3NetworkId,
	}

	wlanIds := [6]string{
		unifi.networkIds.Red1WifiId,
		unifi.networkIds.Red2WifiId,
		unifi.networkIds.Red3WifiId,
		unifi.networkIds.Blue1WifiId,
		unifi.networkIds.Blue2WifiId,
		unifi.networkIds.Blue3WifiId,
	}

	for i := 0; i < 6; i++ {
		// Configure wifi
		if teams[i] == nil {
			continue
		}

		fmt.Printf("Configured wlan for team %d\n", teams[i].Id)
		err := configureWlan(teams[i], networkIds[i], wlanIds[i])
		if err != nil {
			return err
		}
	}

	return nil
}

func (unifi *UnifiClient) unifiGetConfiguredSsids() (*UnifiConfiguredSsids, error) {
	unifi.mutex.Lock()
	defer unifi.mutex.Unlock()

	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("https://%s/api/s/default/rest/wlanconf", unifi.unifiAddress),
		nil)

	if err != nil {
		return nil, fmt.Errorf("creating wifi status request: %w", err)
	}

	resp, err := unifi.client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("executing wifi status request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected wifi status status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading wifi status response body: %w", err)
	}

	var wlanconfigResponse UnifiWlanconfigResponse
	err = json.Unmarshal(body, &wlanconfigResponse)

	if err != nil {
		return nil, fmt.Errorf("parsing wifi status response body: %w", err)
	}

	configuredSsids := UnifiConfiguredSsids{}

	for _, wlan := range wlanconfigResponse.Data {
		if wlan.Id == unifi.networkIds.Red1WifiId {
			configuredSsids.red1Ssid = wlan.Name
		}

		if wlan.Id == unifi.networkIds.Red2WifiId {
			configuredSsids.red2Ssid = wlan.Name
		}

		if wlan.Id == unifi.networkIds.Red3WifiId {
			configuredSsids.red3Ssid = wlan.Name
		}

		if wlan.Id == unifi.networkIds.Blue1WifiId {
			configuredSsids.blue1Ssid = wlan.Name
		}

		if wlan.Id == unifi.networkIds.Blue2WifiId {
			configuredSsids.blue2Ssid = wlan.Name
		}

		if wlan.Id == unifi.networkIds.Blue3WifiId {
			configuredSsids.blue3Ssid = wlan.Name
		}
	}

	return &configuredSsids, nil
}
