package gcp

import (
	"context"
	"errors"
	"github.com/getlantern/lantern-server-privisioner/common"
	"log/slog"
)

type Provisioner struct {
	session      *common.Session
	compartments []common.Compartment
	client       *APIClient
}

func (p *Provisioner) Provision(ctx context.Context, projectID string, locationID string) {
	go func() {
		p.session.Events <- common.Event{Type: common.EventTypeProvisioningStarted, Message: projectID}

		publicSSHKey, privateSSHKey, err := common.MakeSSHKeyPair()
		if err != nil {
			slog.Error("Failed to create SSH key pair", "err", err)
			p.session.Events <- common.Event{Type: common.EventTypeProvisioningError, Error: err}
			return
		}

		if !IsProjectHealthy(ctx, p.client, projectID) {
			p.session.Events <- common.Event{Type: common.EventTypeProvisioningError, Error: errors.New("project is not healthy")}
			slog.Error("Project is not healthy", "projectID", projectID)
			return
		}

		if err := CreateFirewallIfNeeded(ctx, p.client, projectID); err != nil {
			p.session.Events <- common.Event{Type: common.EventTypeProvisioningError, Error: err}
			slog.Error("Failed to create firewall", "err", err)
			return
		}

		instanceName, instanceID, err := CreateInstance(ctx, p.client, projectID, locationID, publicSSHKey)
		if err != nil {
			p.session.Events <- common.Event{Type: common.EventTypeProvisioningError, Error: err}
			slog.Error("Failed to create instance", "err", err)
			return
		}

		instance, err := p.client.GetInstance(ctx, Locator{ProjectID: projectID, ZoneID: locationID, InstanceID: instanceID})
		if err != nil {
			p.session.Events <- common.Event{Type: common.EventTypeProvisioningError, Error: err}
			slog.Error("Failed to get instance", "err", err)
			return
		}
		regionLocator := RegionLocator{ProjectID: projectID, RegionID: GetZoneRegionID(locationID)}
		var ip string
		if sip, err := p.client.GetStaticIP(ctx, regionLocator, instanceName); err != nil {
			natIP := ""
			if len(instance.NetworkInterfaces[0].AccessConfigs) > 0 {
				natIP = instance.NetworkInterfaces[0].AccessConfigs[0].NatIP
			}
			createData := StaticIpCreate{
				Address:     natIP,
				Name:        instanceName,
				Description: instance.Description,
			}
			if sip, err = p.client.CreateStaticIP(ctx, regionLocator, createData); err != nil {
				slog.Error("Failed to create static IP", "err", err)
				p.session.Events <- common.Event{Type: common.EventTypeProvisioningError, Error: err}
				return
			}
			slog.Debug("Created static IP", "ip", sip.Address)
			ip = sip.Address
		} else {
			slog.Debug("Got static IP", "ip", sip)
			ip = sip.Address
		}

		var sc *common.ServerConfiguration
		if sc, err = common.InstallServer(ip, privateSSHKey, "ubuntu"); err != nil {
			slog.Error("Failed to install server", "err", err)
			return
		}

		p.session.Events <- common.Event{
			Type:    common.EventTypeProvisioningCompleted,
			Message: sc.Encode(),
		}
	}()
}

func (p *Provisioner) Validate(ctx context.Context, token string) {
	p.client = NewAPIClient(ctx, token)
	go func(resultChan chan<- common.Event) {
		resultChan <- common.Event{Type: common.EventTypeValidationStarted}
		ba, err := p.client.ListBillingAccounts(ctx)
		if err != nil {
			slog.Error("Failed to list billing accounts", "err", err)
			resultChan <- common.Event{Type: common.EventTypeValidationError, Error: err}
			return
		}
		slog.Debug("Billing accounts retrieved", "accounts", ba)

		for _, account := range ba {
			projects, err := p.client.ListBillingAccountProjects(ctx, account.Name)
			if err != nil {
				slog.Error("Failed to list projects for billing account", "account", account.Name, "err", err)
				resultChan <- common.Event{Type: common.EventTypeValidationError, Error: err}
				return
			}
			slog.Debug("Projects retrieved for billing account", "account", account.Name, "projects", projects)
			if len(projects) == 0 {
				slog.Warn("No projects found for billing account. Skipping", "account", account.Name)
				continue
			}
			var entries []common.CompartmentEntry
			for _, project := range projects {
				zones, err := p.client.ListZones(ctx, project)
				if err != nil {
					slog.Error("Failed to list zones for project. Skipping", "project", project, "err", err)
					continue
				}
				if len(zones) > 0 {
					var res []common.CloudLocation
					for _, zone := range zones {
						// Ensure the zone has a valid location before adding it
						if zone.GetLocation() != nil {
							res = append(res, zone)
						}
					}

					entries = append(entries, common.CompartmentEntry{
						ID:        project,
						Locations: res,
					})
				} else {
					slog.Warn("No zones found for project. Skipping", "project", project)
				}
			}

			p.compartments = append(p.compartments, common.Compartment{
				ID:      account.Name,
				Name:    account.DisplayName,
				Entries: entries,
			})
		}
		resultChan <- common.Event{Type: common.EventTypeValidationCompleted}

	}(p.session.Events)
}

func (p *Provisioner) Session() *common.Session {
	return p.session
}

func (p *Provisioner) Compartments() []common.Compartment {
	return p.compartments
}

func GetProvisioner(ctx context.Context, startBrowser common.BrowserOpener) common.Provisioner {
	session := RunOauth(ctx, startBrowser)
	return &Provisioner{
		session:      session,
		compartments: []common.Compartment{},
		client:       nil,
	}
}
