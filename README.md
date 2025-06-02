# Lantern Server Manager Provisioner

Lantern Server Manager Provisioner is a Go library for provisioning and managing cloud servers across multiple providers, such as DigitalOcean and Google Cloud Platform (GCP). 
It is designed to automate the creation and configuration of infrastructure for the [Lantern Server Manager](https://github.com/getlantern/lantern-server-manager) project.

## Features

- Provision servers for Lantern Server Manager on DigitalOcean and GCP
- Manage server compartments and locations
- OAuth integration for secure API access
- Modular, extensible design for adding new cloud providers

## Project Structure

- `common/` - Shared logic for compartments, installation, location, OAuth, and provisioning
- `digitalocean/` - DigitalOcean-specific API, models, OAuth, and provisioning logic
- `gcp/` - GCP-specific API, models, OAuth, and provisioning logic
- `cmd/main.go` - Example application demonstrating how to use the library

## Usage

See `cmd/main.go` for a complete example.

Notes:
1. The API is asynchronous. Most operations return immediately, and you must monitor the Events channel to track progress.
2. The library uses OAuth for authentication to perform operations on behalf of the user.
3. The library requires a browser to complete the OAuth flow. It will open a browser window for the user to authenticate and authorize access. For desktop applications common.OpenBrowserDesktop can be used. On mobile - please provider your own implementation of `common.BrowserOpener`.
4. After the OAuth flow, we validate that the user has access to the required scopes and has at least one active billing account. If not, an error will be reported.

## Extending

To add support for another cloud provider, implement the required Provisioner API following the existing patterns in `common/`, `digitalocean/`, and `gcp/`.

