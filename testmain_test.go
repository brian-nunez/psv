package psv_test

import (
	"os"
	"testing"

	"github.com/brian-nunez/psv/internal/fakeffmpeg"
)

func TestMain(m *testing.M) {
	os.Exit(fakeffmpeg.Run(m))
}
