// Copyright 2014 Team 254. All Rights Reserved.
// Author: pat@patfairbank.com (Patrick Fairbank)
//
// Methods for configuring a Cisco Switch 3500-series switch for team VLANs.

package network

import (
	"github.com/Team254/cheesy-arena-lite/model"
)

type Switch struct {
	unifi *UnifiClient
}

var ServerIpAddress = "10.0.100.5" // The DS will try to connect to this address only.

func NewSwitch(unifi *UnifiClient) *Switch {
	return &Switch{unifi: unifi}
}

// Sets up wired networks for the given set of teams.
func (sw *Switch) ConfigureTeamEthernet(teams [6]*model.Team) error {
	err := sw.unifi.unifiLogin()

	if err != nil {
		return err
	}

	err = sw.unifi.unifiConfigureNetworks(teams)
	if err != nil {
		return err
	}

	return nil
}
