package slack

import (
	"github.com/goravel/framework/contracts/binding"
	"github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/support/color"
)

type ServiceProvider struct{}

func (r *ServiceProvider) Relationship() binding.Relationship {
	return binding.Relationship{
		Bindings: []string{},
		Dependencies: []string{
			binding.Notification,
			binding.Config,
		},
		ProvideFor: []string{},
	}
}

func (r *ServiceProvider) Register(app foundation.Application) {
	// Nothing to register — see type doc comment.
}

func (r *ServiceProvider) Boot(app foundation.Application) {
	notificationFacade := app.MakeNotification()
	if notificationFacade == nil {
		color.Warningln("Notification Facade is not initialized. Skipping Slack channel registration.")
		return
	}

	config := app.MakeConfig()
	if config == nil {
		color.Warningln("Config Facade is not initialized. Skipping Slack channel registration.")
		return
	}

	token := config.GetString("slack.token")

	notificationFacade.Extend(NewChannel(token))
}
