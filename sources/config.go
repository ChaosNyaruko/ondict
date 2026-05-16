package sources

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/util"
)

// knownCssPrefixes maps dict-name substrings to CSS file stems.
// The first match wins. Entries should be ordered most-specific first.
var knownCssPrefixes = []struct {
	prefix string
	css    string
}{
	{"LDOCE5", "LM5style_vanilla"},
}

type DictConfig struct {
	Name    string `json:"name"`
	Css     string `json:"css"`
	Type    string `json:"type"`
	Enabled *bool  `json:"enabled,omitempty"` // nil means true (backwards compatible)
}

type DefinitionIndexConfig struct {
	Tokenizer string `json:"tokenizer"`
}

type SearchConfig struct {
	DefinitionIndex DefinitionIndexConfig `json:"definition_index"`
}

type Config struct {
	Dicts      []DictConfig `json:"dicts"`
	Search     SearchConfig `json:"search"`
	DefaultCss string       `json:"default_css,omitempty"` // stem of the fallback CSS file (e.g. "LM5style")
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

// resolveCss determines the CSS content for a dictionary.
//
// Priority:
//  1. DictConfig.Css is set → use that file stem.
//  2. Dict name matches a known prefix → use the mapped stem.
//  3. defaultCss (from Config.DefaultCss) is set → use that file stem.
//  4. None of the above, or the resolved file does not exist → return "".
func resolveCss(d DictConfig, defaultCss string) string {
	stem := d.Css
	if stem == "" {
		for _, kv := range knownCssPrefixes {
			if strings.Contains(d.Name, kv.prefix) {
				stem = kv.css
				break
			}
		}
	}
	if stem == "" {
		stem = defaultCss
	}
	if stem == "" {
		return ""
	}
	cssPath := filepath.Join(util.DictsPath(), stem+".css")
	content, err := os.ReadFile(cssPath)
	if err != nil {
		log.Debugf("css file %q not found or unreadable: %v", cssPath, err)
		return ""
	}
	log.Infof("loaded css %q for dict %q", cssPath, d.Name)
	return string(content)
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
		// nil means enabled (backwards compatible with old configs without the field)
		if d.Enabled != nil && !*d.Enabled {
			log.Infof("skipping disabled dict: %v", d.Name)
			continue
		}
		dict := &MdxDict{}
		dict.MdxFile = filepath.Join(util.DictsPath(), d.Name)
		dict.Type = d.Type
		dict.Css = resolveCss(d, c.DefaultCss)
		log.Debugf("get global dict: %v", dict.MdxFile)
		*G = append(*G, dict)
	}
	log.Infof("get global dicts: %v", G)
	return nil
}
