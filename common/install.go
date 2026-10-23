package common

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/crypto/ssh"
)

// MakeSSHKeyPair make a pair of public and private keys for SSH access.
func MakeSSHKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return "", "", err
	}
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}
	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	private := pem.EncodeToMemory(privateKeyPEM)
	public := ssh.MarshalAuthorizedKey(pub)

	return string(public), string(private), nil
}

func establishSSH(ip, privateKey, username string) (*ssh.Client, error) {
	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}
	tryCounter := 0
	maxTries := 10
	var client *ssh.Client
	for {
		client, err = ssh.Dial("tcp", ip+":22", config)
		if err != nil {
			if tryCounter >= maxTries {
				return nil, fmt.Errorf("failed to dial SSH: %w after %d attempts", err, tryCounter)
			}
			tryCounter++
			slog.Debug("SSH connection failed, retrying...", "attempt", tryCounter, "error", err)
			time.Sleep(5 * time.Second)
			continue
		}
		break
	}
	slog.Debug("SSH connection established", "ip", ip)
	return client, nil
}

func runSSHCommand(client *ssh.Client, cmd string) ([]byte, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()
	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to run command %q: %w: %s", cmd, err, output)
	}
	return output, nil
}

type ServerConfiguration struct {
	ExternalIp  string `json:"external_ip"`
	Port        int    `json:"port"`
	AccessToken string `json:"access_token"`
}

func (sc ServerConfiguration) Encode() string {
	data, _ := json.Marshal(sc)
	return string(data)
}

func InstallServer(ip string, privateSSHKey string, username string) (*ServerConfiguration, error) {
	client, err := establishSSH(ip, privateSSHKey, username)
	if err != nil {
		return nil, fmt.Errorf("failed to establish SSH connection: %w", err)
	}
	defer client.Close()

	// Run the installation commands
	commands := []string{
		"sudo cloud-init status --wait",
		"sudo sh -c 'echo deb [trusted=yes] https://apt.fury.io/getlantern/ / > /etc/apt/sources.list.d/getlantern.list'",
		"sudo apt-get update -y -q",
		"sudo apt-get install -y lantern-server-manager sing-box-extensions fail2ban firewalld curl",
		// Load the manager package's cloud-init/firewalld boot-order override.
		"sudo systemctl daemon-reload",
		"sudo systemctl enable --now firewalld",
		"sudo firewall-cmd --add-port 22/tcp --permanent",
		"sudo firewall-cmd --reload",
		"sudo systemctl restart systemd-journald",
		"sudo systemctl enable lantern-server-manager sing-box-extensions",
		"sudo systemctl enable --now fail2ban",
		// The manager writes the proxy config before restarting the proxy itself.
		// Restart explicitly in case the package already tried to start it.
		"sudo systemctl restart lantern-server-manager",
	}
	for _, cmd := range commands {
		output, err2 := runSSHCommand(client, cmd)
		if err2 != nil {
			return nil, err2
		}
		slog.Debug("Command output", "command", cmd, "output", string(output))
	}
	// TODO: add sysctl configuration
	// A Type=simple service can still be initializing after systemctl returns.
	output, err := runSSHCommand(client, "sudo timeout 120 sh -c 'until test -s /opt/lantern/data/server.json; do sleep 1; done; cat /opt/lantern/data/server.json'")
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	slog.Debug("Config file contents", "config", string(output))

	// Parse the JSON output
	var config ServerConfiguration
	if err := json.Unmarshal(output, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	// Probe over the authenticated SSH connection. TLS verification is skipped
	// only for this loopback request, whose certificate is for the public IP.
	if _, err := runSSHCommand(client, fmt.Sprintf("curl --insecure --fail --silent --show-error --retry 30 --retry-all-errors --retry-delay 2 --retry-max-time 120 --max-time 5 https://127.0.0.1:%d/api/v1/health", config.Port)); err != nil {
		return nil, fmt.Errorf("server API did not become ready: %w", err)
	}
	for _, service := range []string{"lantern-server-manager", "sing-box-extensions", "firewalld"} {
		if _, err := runSSHCommand(client, "sudo systemctl is-active --quiet "+service); err != nil {
			return nil, fmt.Errorf("server service %s is not running: %w", service, err)
		}
	}
	return &config, nil
}
