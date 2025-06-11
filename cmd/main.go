package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/pterm/pterm"

	"github.com/getlantern/lantern-server-provisioner/common"
	"github.com/getlantern/lantern-server-provisioner/digitalocean"
)

func testProvisioner(ctx context.Context, p common.Provisioner) {
	session := p.Session()
	for {
		select {
		case e := <-session.Events:
			switch e.Type {
			case common.EventTypeOAuthCompleted:
				slog.Debug("OAuth completed", "token", e.Message)
				// we have the token, now we can proceed
				// this will start the validation process, preparing a list of healthy projects
				// and billing accounts that can be used
				p.Validate(ctx, e.Message)
				continue
			case common.EventTypeOAuthStarted:
				slog.Debug("OAuth started, waiting for user to complete")
			case common.EventTypeOAuthCancelled:
				slog.Debug("OAuth cancelled by user")
				return
			case common.EventTypeOAuthError:
				slog.Error("OAuth failed", "err", e.Error)
				return
			case common.EventTypeValidationStarted:
				slog.Debug("Validation started")
			case common.EventTypeValidationCompleted:
				// at this point we have a list of projects and billing accounts
				// present them to the user
				slog.Debug("Validation completed, ready to create resources")
				compartments := p.Compartments()
				if len(compartments) == 0 {
					slog.Error("No valid projects found, please check your billing account and permissions")
					return
				}
				slog.Debug("Available projects", "compartments", compartments)
				selectedAccount, _ := pterm.DefaultInteractiveSelect.WithOptions(common.CompartmentNames(compartments)).Show("Please select an account:")
				pterm.Info.Printfln("Selected option: %s", pterm.Green(selectedAccount))
				compartment := common.CompartmentByName(compartments, selectedAccount)

				selectedProject, _ := pterm.DefaultInteractiveSelect.WithOptions(common.CompartmentEntryIDs(compartment.Entries)).Show("Please select a project:")
				pterm.Info.Printfln("Selected project: %s", pterm.Green(selectedProject))

				project := common.CompartmentEntryByID(compartment.Entries, selectedProject)
				selectedLocation, _ := pterm.DefaultInteractiveSelect.WithOptions(common.CompartmentEntryLocations(project)).Show("Please select a location:")
				pterm.Info.Printfln("Selected location: %s", pterm.Green(selectedLocation))

				// we can now proceed to create resources
				cloc := common.CompartmentLocationByIdentifier(project.Locations, selectedLocation)
				p.Provision(ctx, selectedProject, cloc.GetID())

			case common.EventTypeValidationError:
				slog.Error("Validation failed", "err", e.Error)
				return
			case common.EventTypeProvisioningStarted:
				slog.Debug("Provisioning started")
			case common.EventTypeProvisioningCompleted:
				slog.Debug("Provisioning completed successfully", "result", e.Message)
				return
			case common.EventTypeProvisioningError:
				slog.Error("Provisioning failed", "err", e.Error)
				return
			}
			break
		default:
			time.Sleep(1 * time.Second)
		}
	}
}

// Example Usage
func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	ctx := context.Background()
	//p := gcp.GetProvisioner(ctx, Start)
	p := digitalocean.GetProvisioner(ctx, common.OpenBrowserDesktop)
	testProvisioner(ctx, p)
}
