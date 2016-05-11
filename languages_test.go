package gfmxr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	fakeLanguages = []byte(`
Fribble:
  type: programming
  color: "#FF0000"
  extensions:
  - .frb
  ace_mode: fribble

Snarf:
  type: programming
  color: "#00FF00"
  extensions:
  - .snarf
  aliases:
  - snarrf
  - snarrrf
  ace_mode: python
  group: Python
`)
)

func TestLoadLanguagesFromBytes(t *testing.T) {
	langs, err := loadLanguagesFromBytes(fakeLanguages)
	assert.Nil(t, err)
	assert.NotNil(t, langs)
	assert.Contains(t, langs.Map, "snarf")
	assert.Contains(t, langs.Map, "snarrf")
	assert.Contains(t, langs.Map, "snarrrf")
	assert.Contains(t, langs.Map, "fribble")
}
