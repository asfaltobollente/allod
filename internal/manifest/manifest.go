package manifest

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Manifest struct {
	ID          string            `yaml:"id" json:"id"`
	Tier        string            `yaml:"tier" json:"tier"`
	Provides    []string          `yaml:"provides" json:"provides"`
	Conflicts   []string          `yaml:"conflicts" json:"conflicts"`
	Platforms   []string          `yaml:"platforms" json:"platforms"`
	Levels      map[string]Level  `yaml:"levels" json:"levels"`
	Ports       []Port            `yaml:"ports" json:"ports"`
	Privileges  Privileges        `yaml:"privileges" json:"privileges"`
	Images      []Image           `yaml:"images" json:"images"`
	Update      UpdateConfig      `yaml:"update" json:"update"`
	Transitions map[string]string `yaml:"transitions" json:"transitions"`
	Datasets    []Dataset         `yaml:"datasets" json:"datasets"`
	Help        map[string]string `yaml:"help" json:"help"`
}

type Level struct {
	RAMMB      int      `yaml:"ram_mb" json:"ram_mb"`
	DiskGB     int      `yaml:"disk_gb,omitempty" json:"disk_gb,omitempty"`
	Requires   Requires `yaml:"requires,omitempty" json:"requires,omitempty"`
	Grants     []string `yaml:"grants,omitempty" json:"grants,omitempty"`
	Lacks      []string `yaml:"lacks,omitempty" json:"lacks,omitempty"`
	NotesARM64 string   `yaml:"notes_arm64,omitempty" json:"notes_arm64,omitempty"`
}

type Requires struct {
	Modules     []string `yaml:"modules,omitempty" json:"modules,omitempty"`
	CPUFlags    []string `yaml:"cpu_flags,omitempty" json:"cpu_flags,omitempty"`
	TotalRAMMB  int      `yaml:"total_ram_mb,omitempty" json:"total_ram_mb,omitempty"`
	VideoEncode []string `yaml:"video_encode,omitempty" json:"video_encode,omitempty"`
}

type Port struct {
	N     int    `yaml:"n" json:"n"`
	Scope string `yaml:"scope" json:"scope"`
	Share string `yaml:"share,omitempty" json:"share,omitempty"`
}

type Privileges struct {
	Userns  string   `yaml:"userns" json:"userns"`
	Devices []string `yaml:"devices,omitempty" json:"devices,omitempty"`
	Caps    []string `yaml:"caps,omitempty" json:"caps,omitempty"`
}

type Image struct {
	Ref     string   `yaml:"ref" json:"ref"`
	Tag     string   `yaml:"tag" json:"tag"`
	Channel string   `yaml:"channel" json:"channel"`
	Args    []string `yaml:"args,omitempty" json:"args,omitempty"`
}

type UpdateConfig struct {
	RequiresReleaseNotes bool   `yaml:"requires_release_notes" json:"requires_release_notes"`
	PreHook              string `yaml:"pre_hook,omitempty" json:"pre_hook,omitempty"`
	BreaksMobileApps     bool   `yaml:"breaks_mobile_apps" json:"breaks_mobile_apps"`
}

type Dataset struct {
	ID       string   `yaml:"id" json:"id"`
	Includes []string `yaml:"includes,omitempty" json:"includes,omitempty"`
	Excludes []string `yaml:"excludes,omitempty" json:"excludes,omitempty"`
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
