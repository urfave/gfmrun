package gfmxr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRunnerIntegration(t *testing.T) {
	if !integrationTests {
		t.Skip("integration tests disabled")
	}

	runner, err := NewRunner([]string{"README.md"}, 0, "", true, testLog)
	assert.Nil(t, err)
	assert.NotNil(t, runner.Languages)
	assert.Equal(t, []string{"README.md"}, runner.Sources)
	assert.NotNil(t, runner.Frobs)
	assert.Equal(t, 0, runner.Count)
}
