package cloudinit

import (
	"bytes"
	"fmt"
	"regexp"
	"text/template"
)

var hostnameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

const tpl = `#cloud-config
autoinstall:
  version: 1
  identity:
    hostname: {{.Hostname}}
    password: "$6$ex$m1..." # Dummy pass, never used directly
    username: allod
  packages:
    - podman
    - btrfs-progs
  runcmd:
    - echo "Impostazione repository Allod..."
    - wget -O /usr/share/keyrings/allod.gpg https://dl.allod.dev/key.gpg
    - echo "deb [signed-by=/usr/share/keyrings/allod.gpg] https://dl.allod.dev/apt stable main" > /etc/apt/sources.list.d/allod.list
    - apt-get update && apt-get install -y allod-core
    - systemctl enable --now allod-panel
`

type Config struct {
	Hostname string
}

func Generate(cfg Config) (string, error) {
	if !hostnameRegex.MatchString(cfg.Hostname) {
		return "", fmt.Errorf("invalid hostname %q: must be 1-63 chars, alphanumeric or hyphens, matching RFC 1123", cfg.Hostname)
	}

	t, err := template.New("cloudinit").Parse(tpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, cfg); err != nil {
		return "", err
	}

	return buf.String(), nil
}
