package gfmrun

import (
	"io/ioutil"
	"os"

	"github.com/Sirupsen/logrus"
)

var (
	integrationTests = os.Getenv("GFMRUN_DISABLE_INTEGRATION_TESTS") == ""
	testLog          = logrus.New()
)

func init() {
	testLog.Level = logrus.PanicLevel
	testLog.Out = ioutil.Discard
}
