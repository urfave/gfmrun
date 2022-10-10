package gfmrun

import (
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

var (
	integrationTests = os.Getenv("GFMRUN_DISABLE_INTEGRATION_TESTS") == ""
	testLog          = logrus.New()
)

func init() {
	testLog.Level = logrus.PanicLevel
	testLog.Out = io.Discard
}
