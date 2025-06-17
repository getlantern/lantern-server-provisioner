package digitalocean

import (
	"context"
	"log/slog"

	"github.com/getlantern/lantern-server-provisioner/common"
)

type Provisioner struct {
	accounts []common.Compartment
	client   *APIClient
	session  *common.Session
}

func GetProvisioner(ctx context.Context, browserStart common.BrowserOpener) *Provisioner {
	session := RunOauth(ctx, browserStart)
	return &Provisioner{
		session: session,
	}
}

func (p *Provisioner) Validate(ctx context.Context, token string) {
	p.client = NewRestApiSession(token, nil)
	go func(resultChan chan<- common.Event) {

		resultChan <- common.Event{Type: common.EventTypeValidationStarted}

		account, err := GetAccount(token)
		if err != nil {
			slog.Error("Failed to get account info with token", "err", err)
			resultChan <- common.Event{Type: common.EventTypeValidationError, Error: err}
			return
		}

		projects, err := p.client.GetProjects(ctx)
		if err != nil {
			slog.Error("Failed to get projects", "err", err)
			resultChan <- common.Event{Type: common.EventTypeValidationError, Error: err}
			return
		}
		regionInfo, err := p.client.GetRegionInfo(ctx)
		if err != nil {
			slog.Error("Failed to get region info", "err", err)
			resultChan <- common.Event{Type: common.EventTypeValidationError, Error: err}
			return
		}
		var locs []common.CloudLocation
		for _, ri := range regionInfo {
			if ri.Available {
				locs = append(locs, ri)
			}
		}
		var entries []common.CompartmentEntry
		for _, project := range projects {
			entries = append(entries, common.CompartmentEntry{
				ID:        project.Id,
				Locations: locs,
			})
		}
		p.accounts = []common.Compartment{
			{
				ID:      account.UUID,
				Name:    account.Email,
				Entries: entries,
			},
		}
		slog.Debug("Validation completed", "account", account.Email, "projects", len(entries), "locations", len(locs))
		resultChan <- common.Event{Type: common.EventTypeValidationCompleted}
	}(p.session.Events)
}

func (p *Provisioner) Compartments() []common.Compartment {
	return p.accounts
}

func (p *Provisioner) Session() *common.Session {
	return p.session
}

func (p *Provisioner) Provision(ctx context.Context, placementID string, locationID string) {
	go func() {
		p.session.Events <- common.Event{Type: common.EventTypeProvisioningStarted, Message: placementID}

		publicSSHKey, privateSSHKey, err := common.MakeSSHKeyPair()
		if err != nil {
			slog.Error("Failed to create SSH key pair", "err", err)
			p.session.Events <- common.Event{Type: common.EventTypeProvisioningError, Error: err}
			return
		}

		name := common.MakeInstanceName()
		if di, err := p.client.CreateDroplet(ctx, name, locationID, publicSSHKey, DropletSpecification{
			Size:  "s-1vcpu-1gb",
			Image: "ubuntu-22-04-x64",
		}); err != nil {
			slog.Error("Failed to create droplet", "err", err)
			p.session.Events <- common.Event{Type: common.EventTypeProvisioningError, Error: err}
			return
		} else {
			slog.Debug("Created droplet", "dropletID", di.ID)
			dropletID := di.ID
			ip := di.Networks.V4[0].IPAddress
			if conf, err := common.InstallServer(ip, privateSSHKey, "root"); err != nil {
				slog.Error("Failed to install server", "err", err)
				p.session.Events <- common.Event{Type: common.EventTypeProvisioningError, Error: err}
				return
			} else {
				slog.Debug("Installed server on droplet", "id", dropletID, "conf", conf)
				p.session.Events <- common.Event{
					Type:    common.EventTypeProvisioningCompleted,
					Message: conf.Encode(),
				}
			}
		}
	}()
}
