package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/beihehele/subs-check/config"
)

func TestConfigReloadClearsRemovedRulesAndRejectsInvalid(t *testing.T) {
	old := config.GlobalConfig
	config.GlobalConfig = config.Defaults()
	t.Cleanup(func() { config.GlobalConfig = old })
	path := filepath.Join(t.TempDir(), "config.yaml")
	app := &App{configPath: path}
	load := func(data string) error {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		return app.loadConfig()
	}
	if err := load("selection: {region-weights: {HK: 2}}\nnode-filter: {regions: [HK]}"); err != nil {
		t.Fatal(err)
	}
	if err := load("selection: {region-weights: {HK: 9}}\nfilter: ['[']"); err == nil {
		t.Fatal("invalid config accepted")
	}
	if config.GlobalConfig.Selection.RegionWeights["HK"] != 2 {
		t.Fatal("failed reload mutated live config")
	}
	if err := load("rename-node: true"); err != nil {
		t.Fatal(err)
	}
	if len(config.GlobalConfig.Selection.RegionWeights) != 0 || len(config.GlobalConfig.NodeFilter.Regions) != 0 {
		t.Fatal("removed rules retained")
	}
}
