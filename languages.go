package gfmrun

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

var (
	DefaultLanguagesYml    = filepath.Join(getCacheDir(), "languages.yml")
	DefaultLanguagesYmlURL = "https://raw.githubusercontent.com/github/linguist/master/lib/linguist/languages.yml"
)

type Languages struct {
	Map map[string]*LanguageDefinition
}

func (l *Languages) denormalize() {
	for key, def := range l.Map {
		func() {
			lowkey := strings.ToLower(key)
			def.Name = lowkey

			if def.Aliases == nil {
				def.Aliases = []string{}
			}

			for _, alias := range def.Aliases {
				if alias == lowkey {
					return
				}
			}

			def.Aliases = append(def.Aliases, lowkey)
		}()

		for _, alias := range def.Aliases {
			l.Map[alias] = &LanguageDefinition{
				Name:         alias,
				Type:         def.Type,
				Aliases:      []string{},
				Interpreters: def.Interpreters,
				AceMode:      def.AceMode,
				Group:        def.Group,
				Canonical:    def,
			}
		}

		l.Map[def.Name] = def
	}
}

func (l *Languages) Lookup(identifier string) *LanguageDefinition {
	lowerIdent := strings.ToLower(identifier)
	for key, def := range l.Map {
		if key == lowerIdent {
			return selfOrCanonical(def)
		}
	}

	for _, def := range l.Map {
		if def.Aliases == nil {
			def.Aliases = []string{}
		}

		for _, alias := range def.Aliases {
			if alias == lowerIdent {
				return selfOrCanonical(def)
			}
		}

		if def.AceMode != "" && def.AceMode == lowerIdent {
			return selfOrCanonical(def)
		}

		if def.Group != "" && strings.ToLower(def.Group) == lowerIdent {
			return selfOrCanonical(def)
		}
	}

	return nil
}

func selfOrCanonical(def *LanguageDefinition) *LanguageDefinition {
	if def.Canonical != nil {
		return def.Canonical
	}

	return def
}

type LanguageDefinition struct {
	Name         string              `json:"-"`
	Type         string              `json:"type,omitempty" yaml:"type"`
	Aliases      []string            `json:"aliases,omitempty" yaml:"aliases"`
	Interpreters []string            `json:"interpreters,omitempty" yaml:"interpreters"`
	AceMode      string              `json:"ace_mode,omitempty" yaml:"ace_mode"`
	Group        string              `json:"group,omitempty" yaml:"group"`
	Canonical    *LanguageDefinition `json:"-"`
}

func LoadLanguages(languagesYml string) (*Languages, error) {
	if languagesYml == "" {
		languagesYml = DefaultLanguagesYml
	}

	rawBytes, err := os.ReadFile(languagesYml)
	if err != nil {
		return nil, err
	}

	return loadLanguagesFromBytes(rawBytes)
}

func loadLanguagesFromBytes(languagesYmlBytes []byte) (*Languages, error) {
	m := map[string]*LanguageDefinition{}
	err := yaml.Unmarshal(languagesYmlBytes, &m)
	if err != nil {
		return nil, err
	}

	langs := &Languages{Map: m}
	langs.denormalize()
	return langs, nil
}

func PullLanguagesYml(srcURL, destFile string) error {
	if srcURL == "" {
		srcURL = DefaultLanguagesYmlURL
	}

	if destFile == "" {
		destFile = DefaultLanguagesYml
	}

	err := os.MkdirAll(filepath.Dir(destFile), os.FileMode(0750))
	if err != nil {
		return err
	}

	resp, err := http.Get(srcURL)
	if err != nil {
		return err
	}

	if resp.StatusCode > 299 {
		return fmt.Errorf("fetching %q returned status %v", srcURL, resp.StatusCode)
	}

	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	outTmp, err := os.CreateTemp("", "gfmrun-linguist")
	if err != nil {
		_ = outTmp.Close()
		return err
	}

	_ = outTmp.Close()

	defer func() { _ = os.Remove(outTmp.Name()) }()

	err = os.WriteFile(outTmp.Name(), respBodyBytes, os.FileMode(0640))
	if err != nil {
		return err
	}

	// Atomic copy of downloaded language file
	in, err := os.Open(outTmp.Name())
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
