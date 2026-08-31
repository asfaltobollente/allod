package manifest

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Manifest struct {
	ID          string                `yaml:"id"`
	Tier        string                `yaml:"tier"`
	Provides    []string              `yaml:"provides"`
	Conflicts   []string              `yaml:"conflicts"`
	Platforms   []string              `yaml:"platforms"`
	Levels      map[string]Level      `yaml:"levels"`
	Ports       []Port                `yaml:"ports"`
	Privileges  Privileges            `yaml:"privileges"`
	Images      []Image               `yaml:"images"`
	Update      UpdateConfig          `yaml:"update"`
	Transitions map[string]string     `yaml:"transitions"`
	Datasets    []Dataset             `yaml:"datasets"`
	Help        map[string]string     `yaml:"help"`
}

type Level struct {
	RAMMB       int      `yaml:"ram_mb"`
	DiskGB      int      `yaml:"disk_gb,omitempty"`
	Requires    Requires `yaml:"requires,omitempty"`
	Grants      []string `yaml:"grants,omitempty"`
	Lacks       []string `yaml:"lacks,omitempty"`
	NotesARM64  string   `yaml:"notes_arm64,omitempty"`
}

type Requires struct {
	Modules     []string `yaml:"modules,omitempty"`
	CPUFlags    []string `yaml:"cpu_flags,omitempty"`
	TotalRAMMB  int      `yaml:"total_ram_mb,omitempty"`
	VideoEncode []string `yaml:"video_encode,omitempty"`
}

type Port struct {
	N     int    `yaml:"n"`
	Scope string `yaml:"scope"`
	Share string `yaml:"share,omitempty"`
}

type Privileges struct {
	Userns  string   `yaml:"userns"`
	Devices []string `yaml:"devices,omitempty"`
	Caps    []string `yaml:"caps,omitempty"`
}

type Image struct {
	Ref     string   `yaml:"ref"`
	Tag     string   `yaml:"tag"`
	Channel string   `yaml:"channel"`
	Args    []string `yaml:"args,omitempty"`
}

type UpdateConfig struct {
	RequiresReleaseNotes bool   `yaml:"requires_release_notes"`
	PreHook              string `yaml:"pre_hook,omitempty"`
	BreaksMobileApps     bool   `yaml:"breaks_mobile_apps"`
}

type Dataset struct {
	ID       string   `yaml:"id"`
	Includes []string `yaml:"includes,omitempty"`
	Excludes []string `yaml:"excludes,omitempty"`
}

func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var m Manifest
	err = yaml.Unmarshal(data, &m)
	if err != nil {
		return nil, err
	}

	return &m, nil
}
