package sources

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/util"
)

type DictConfig struct {
	Name string `json:"name"`
	Css  string `json:"css"`
	Type string `json:"type"`
}

type DefinitionIndexConfig struct {
	Tokenizer string `json:"tokenizer"`
}

type SearchConfig struct {
	DefinitionIndex DefinitionIndexConfig `json:"definition_index"`
}

type Config struct {
	Dicts  []DictConfig `json:"dicts"`
	Search SearchConfig `json:"search"`
}

func DefaultConfig() Config {
	return Config{
		Dicts: nil,
		Search: SearchConfig{
			DefinitionIndex: DefinitionIndexConfig{
				Tokenizer: "unicode61",
			},
		},
	}
}

func normalizeConfig(c *Config) {
	if c.Search.DefinitionIndex.Tokenizer == "" {
		c.Search.DefinitionIndex.Tokenizer = "unicode61"
	}
}

func ReadConfig() (Config, error) {
	data, err := os.ReadFile(filepath.Join(util.ConfigPath(), "config.json"))
	if err != nil && errors.Is(err, os.ErrNotExist) {
		log.Debugf("load config file err: %v, default settings are used.", err)
		return DefaultConfig(), err
	}
	c := DefaultConfig()
	if err := json.Unmarshal(data, &c); err != nil {
		log.Errorf("bad json unmarshal: %v, default settings are used.", err)
		return c, err
	}
	normalizeConfig(&c)
	return c, nil
}

func LoadConfig() error {
	c, err := ReadConfig()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(c.Dicts) == 0 {
		return nil
	}
	for _, d := range c.Dicts {
		dict := &MdxDict{}
		dict.MdxFile = filepath.Join(util.DictsPath(), d.Name)
		dict.Type = d.Type
		log.Debugf("get global dict: %v", dict.MdxFile)
		*G = append(*G, dict)
	}
	log.Infof("get global dicts: %v", G)
	return nil
}
