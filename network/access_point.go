// Copyright 2017 Team 254. All Rights Reserved.
// Author: pat@patfairbank.com (Patrick Fairbank)
//
// Methods for configuring a Linksys WRT1900ACS access point running OpenWRT for team SSIDs and VLANs.

package network

import (
	"fmt"
	"github.com/Team254/cheesy-arena-lite/model"
	"log"
	"strconv"
	"time"
)

const (
	accessPointSshPort                = 22
	accessPointConnectTimeoutSec      = 1
	accessPointCommandTimeoutSec      = 5
	accessPointPollPeriodSec          = 3
	accessPointRequestBufferSize      = 10
	accessPointConfigRetryIntervalSec = 5
)

type AccessPoint struct {
	unifi                  *UnifiClient
	configRequestChan      chan [6]*model.Team
	TeamWifiStatuses       [6]TeamWifiStatus
	initialStatusesFetched bool
}

type TeamWifiStatus struct {
	TeamId      int
	RadioLinked bool
}

type sshOutput struct {
	output string
	err    error
}

func (ap *AccessPoint) SetSettings(unifi *UnifiClient) {
	ap.unifi = unifi

	// Create config channel the first time this method is called.
	if ap.configRequestChan == nil {
		ap.configRequestChan = make(chan [6]*model.Team, accessPointRequestBufferSize)
	}
}

// Loops indefinitely to read status from and write configurations to the access point.
func (ap *AccessPoint) Run() {
	for {
		// Check if there are any pending configuration requests; if not, periodically poll wifi status.
		select {
		case request := <-ap.configRequestChan:
			// If there are multiple requests queued up, only consider the latest one.
			numExtraRequests := len(ap.configRequestChan)
			for i := 0; i < numExtraRequests; i++ {
				request = <-ap.configRequestChan
			}
			ap.handleTeamWifiConfiguration(request)
		case <-time.After(time.Second * accessPointPollPeriodSec):
			ap.updateTeamWifiStatuses()
		}
	}
}

// Adds a request to set up wireless networks for the given set of teams to the asynchronous queue.
func (ap *AccessPoint) ConfigureTeamWifi(teams [6]*model.Team) error {
	// Use a channel to serialize configuration requests; the monitoring goroutine will service them.
	select {
	case ap.configRequestChan <- teams:
		return nil
	default:
		return fmt.Errorf("WiFi config request buffer full")
	}
}

func (ap *AccessPoint) ConfigureAdminWifi() error {
	return nil
}

func (ap *AccessPoint) handleTeamWifiConfiguration(teams [6]*model.Team) {
	fmt.Printf("%v\n", teams)
	if ap.configIsCorrectForTeams(teams) {
		return
	}

	// Generate the configuration command.

	// Loop indefinitely at writing the configuration and reading it back until it is successfully applied.
	attemptCount := 1
	for {
		err := ap.unifi.unifiLogin()
		if err == nil {
			err := ap.unifi.unifiConfigureWifi(teams)
			if err != nil {
				log.Printf("Error configuring WiFi: %v", err)
			}
		}

		// Wait before reading the config back on write success as it doesn't take effect right away, or before retrying
		// on failure.
		time.Sleep(time.Second * accessPointConfigRetryIntervalSec)

		if err == nil {
			err = ap.updateTeamWifiStatuses()
			if err == nil && ap.configIsCorrectForTeams(teams) {
				log.Printf("Successfully configured WiFi after %d attempts.", attemptCount)
				return
			}
		}

		if err != nil {
		}

		log.Printf("WiFi configuration still incorrect after %d attempts; trying again.", attemptCount)
		attemptCount++
	}
}

// Returns true if the configured networks as read from the access point match the given teams.
func (ap *AccessPoint) configIsCorrectForTeams(teams [6]*model.Team) bool {
	if !ap.initialStatusesFetched {
		return false
	}

	for i, team := range teams {
		expectedTeamId := 0
		if team != nil {
			expectedTeamId = team.Id
		}
		if team != nil && ap.TeamWifiStatuses[i].TeamId != expectedTeamId {
			return false
		}
	}

	return true
}

// Fetches the current wifi network status from the access point and updates the status structure.
func (ap *AccessPoint) updateTeamWifiStatuses() error {
	err := ap.unifi.unifiLogin()
	if err != nil {
		return err
	}

	output, err := ap.unifi.unifiGetConfiguredSsids()

	if err != nil {
		return fmt.Errorf("Error getting wifi info from AP: %v", err)
	} else {
		ap.TeamWifiStatuses[0].TeamId, _ = strconv.Atoi(output.red1Ssid)
		ap.TeamWifiStatuses[1].TeamId, _ = strconv.Atoi(output.red2Ssid)
		ap.TeamWifiStatuses[2].TeamId, _ = strconv.Atoi(output.red3Ssid)
		ap.TeamWifiStatuses[3].TeamId, _ = strconv.Atoi(output.blue1Ssid)
		ap.TeamWifiStatuses[4].TeamId, _ = strconv.Atoi(output.blue2Ssid)
		ap.TeamWifiStatuses[5].TeamId, _ = strconv.Atoi(output.blue3Ssid)

		if !ap.initialStatusesFetched {
			ap.initialStatusesFetched = true
		}
	}
	return nil
}
