package full

import (
	"fmt"
	"github.com/stretchr/testify/require"
	integration "gitlab.com/shar-workflow/shar/cli/ingtegration"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"os"
	"testing"
)

var tst *support.Integration

type Empty struct{}

func TestMain(m *testing.M) {
	packageNameStruct := Empty{}

	packageName := support.GetPackageName(packageNameStruct)
	fmt.Printf("\033[1;36m%s\033[0m", "> start tests for "+packageName+"\n")

	tst = support.NewIntegration(false, packageName, nil)
	tst.Setup()

	code := m.Run()
	defer func() {
		tst.Teardown()
		fmt.Printf("\033[1;36m%s\033[0m", "> end tests for "+packageName+"\n")
		os.Exit(code)
	}()
}

func cmd[rt any](t *testing.T, line string) rt { // nolint
	fmt.Println("$", line)
	res, err := integration.ExecTst[rt](t, tst, line)
	require.NoError(t, err)
	return res
}
