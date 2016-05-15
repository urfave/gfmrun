package gfmxr

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	integrationTests = os.Getenv("DISABLE_INTEGRATION_TESTS") == ""

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

func TestLanguages_Lookup(t *testing.T) {
	langs, err := loadLanguagesFromBytes(fakeLanguages)
	assert.Nil(t, err)

	assert.Equal(t, "Python", langs.Lookup("snarrrf").Group)
	assert.Equal(t, "Python", langs.Lookup("snarrf").Group)
	assert.Equal(t, "Python", langs.Lookup("snarf").Group)
	assert.Equal(t, "snarf", langs.Lookup("snarrrf").Name)
	assert.Equal(t, "snarf", langs.Lookup("snarrf").Name)
	assert.Equal(t, "snarf", langs.Lookup("snarf").Name)
	assert.Equal(t, "fribble", langs.Lookup("fribble").AceMode)
	assert.Equal(t, "fribble", langs.Lookup("fribble").Name)
	assert.Nil(t, langs.Lookup("flurb"))
}

func TestLanguagesIntegration(t *testing.T) {
	if !integrationTests {
		return
	}

	tf, err := ioutil.TempFile("", "gfmxr-test")
	assert.Nil(t, err)
	tf.Close()

	defer func() { _ = os.Remove(tf.Name()) }()

	err = PullLanguagesYml("", tf.Name())
	assert.Nil(t, err)

	langs, err := LoadLanguages(tf.Name())
	assert.Nil(t, err)

	assert.Equal(t, "xquery", langs.Lookup("xquery").AceMode)
	assert.Equal(t, "xml", langs.Lookup("wsdl").Name)
	assert.Equal(t, "text", langs.Lookup("vbnet").AceMode)
}
