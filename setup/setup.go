package main

import (
	"os"

	"github.com/goravel/framework/packages"
	"github.com/goravel/framework/packages/modify"
)

func main() {
	setup := packages.Setup(os.Args)
	serviceProvider := "&slack.ServiceProvider{}"
	moduleImport := "slack " + setup.Paths().Module().Import()

	setup.Install(
		modify.RegisterProvider(moduleImport, serviceProvider),
	).Uninstall(
		modify.UnregisterProvider(moduleImport, serviceProvider),
	).Execute()
}
