package gfmxr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCLI(t *testing.T) {
	app := NewCLI()
	assert.NotNil(t, app)
	assert.Equal(t, "gfmxr", app.Name)
	assert.NotEmpty(t, app.Usage)
	assert.NotEmpty(t, app.Authors)
	assert.NotEmpty(t, app.Version)
	assert.NotEmpty(t, app.Flags)
	assert.NotEmpty(t, app.Commands)
	assert.NotNil(t, app.Action)
}
