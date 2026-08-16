package main

import (
	"os"

	"github.com/goravel/framework/packages"
	"github.com/goravel/framework/packages/modify"
	"github.com/goravel/framework/support/path"
)

func main() {
	setup := packages.Setup(os.Args)
	stubs := Stubs{}

	serviceProvider := "&slack.ServiceProvider{}"
	moduleImport := "slack " + setup.Paths().Module().Import()
	configPath := path.Config("slack.go")

	configPackage := setup.Paths().Config().Package()
	facadesImport := setup.Paths().Facades().Import()
	facadesPackage := setup.Paths().Facades().Package()

	setup.Install(
		modify.RegisterProvider(moduleImport, serviceProvider),
		modify.File(configPath).Overwrite(stubs.Config(configPackage, facadesImport, facadesPackage)),
	).Uninstall(
		modify.File(configPath).Remove(),
		modify.UnregisterProvider(moduleImport, serviceProvider),
	).Execute()
}
