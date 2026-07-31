package slack

import (
	"github.com/goravel/framework/contracts/binding"
	"github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/support/color"
)

const modulePath = "github.com/goravel/slack"

// ServiceProvider extends the notification.Manager with the Slack
// channel — it registers nothing of its own in the container, only
// calling Extend() on a binding (Notification) owned by a different
// module. Register() is intentionally a no-op.

type ServiceProvider struct{}

func (r *ServiceProvider) Relationship() binding.Relationship {
	return binding.Relationship{
		Bindings: []string{},
		Dependencies: []string{
			binding.Notification,
			binding.Http,
			binding.Config,
		},
		ProvideFor: []string{},
	}
}

func (r *ServiceProvider) Register(app foundation.Application) {
	// Nothing to register
}

func (r *ServiceProvider) Boot(app foundation.Application) {
	app.Publishes(modulePath, map[string]string{
		"config/slack.go": app.ConfigPath("slack.go"),
	})

	notificationFacade := app.MakeNotification()
	if notificationFacade == nil {
		color.Warningln("Notification Facade is not initialized. Skipping Slack channel registration.")
		return
	}

	httpFacade := app.MakeHttp()
	if httpFacade == nil {
		color.Warningln("Http Facade is not initialized. Skipping Slack channel registration.")
		return
	}

	config := app.MakeConfig()
	if config == nil {
		color.Warningln("Config Facade is not initialized. Skipping Slack channel registration.")
		return
	}

	token := config.GetString("slack.token")

	notificationFacade.Extend(NewChannel(httpFacade, token))
}
