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
	Name string
	Css  string
	Type string
}

type Config struct {
	Dicts []DictConfig `json:"dicts"`
}

func LoadConfig() error {
	data, err := os.ReadFile(filepath.Join(util.ConfigPath(), "config.json"))
	if err != nil && errors.Is(err, os.ErrNotExist) {
		log.Debugf("load config file err: %v, default settings are used.", err)
		return err
	}
	c := Config{}
	if err := json.Unmarshal(data, &c); err != nil {
		log.Errorf("bad json unmarshal: %v, default settings are used.", err)
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
