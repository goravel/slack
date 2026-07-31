package slack

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/goravel/framework/contracts/binding"
	mocksconfig "github.com/goravel/framework/mocks/config"
	mocksfoundation "github.com/goravel/framework/mocks/foundation"
	mocksclient "github.com/goravel/framework/mocks/http/client"
	mocksnotification "github.com/goravel/framework/mocks/notification"
)

func TestServiceProviderRelationship(t *testing.T) {
	provider := &ServiceProvider{}
	rel := provider.Relationship()

	assert.Empty(t, rel.Bindings)
	assert.Contains(t, rel.Dependencies, binding.Notification)
	assert.Contains(t, rel.Dependencies, binding.Http)
	assert.Contains(t, rel.Dependencies, binding.Config)
	assert.Empty(t, rel.ProvideFor)
}

func TestServiceProviderRegister_IsNoOp(t *testing.T) {
	provider := &ServiceProvider{}
	app := mocksfoundation.NewApplication(t) // no calls expected

	assert.NotPanics(t, func() {
		provider.Register(app)
	})
}

// expectPublishes sets up the two calls Boot() now makes
// unconditionally, right at the top, before any of the facade
// nil-checks — every Boot() test needs this regardless of which
// branch it's exercising afterward.
func expectPublishes(app *mocksfoundation.Application) {
	app.EXPECT().ConfigPath("slack.go").Return("config/slack.go").Once()
	app.EXPECT().Publishes(modulePath, map[string]string{"config/slack.go": "config/slack.go"}).Once()
}

func TestServiceProviderBoot_SkipsWhenNotificationFacadeNotSet(t *testing.T) {
	provider := &ServiceProvider{}
	app := mocksfoundation.NewApplication(t)
	expectPublishes(app)
	app.EXPECT().MakeNotification().Return(nil).Once()

	assert.NotPanics(t, func() {
		provider.Boot(app)
	})
}

func TestServiceProviderBoot_SkipsWhenHttpFacadeNotSet(t *testing.T) {
	provider := &ServiceProvider{}
	app := mocksfoundation.NewApplication(t)
	expectPublishes(app)
	mgr := mocksnotification.NewManager(t)
	app.EXPECT().MakeNotification().Return(mgr).Once()
	app.EXPECT().MakeHttp().Return(nil).Once()

	assert.NotPanics(t, func() {
		provider.Boot(app)
	})
}

func TestServiceProviderBoot_SkipsWhenConfigFacadeNotSet(t *testing.T) {
	provider := &ServiceProvider{}
	app := mocksfoundation.NewApplication(t)
	expectPublishes(app)
	mgr := mocksnotification.NewManager(t)
	http := mocksclient.NewFactory(t)
	app.EXPECT().MakeNotification().Return(mgr).Once()
	app.EXPECT().MakeHttp().Return(http).Once()
	app.EXPECT().MakeConfig().Return(nil).Once()

	assert.NotPanics(t, func() {
		provider.Boot(app)
	})
}

func TestServiceProviderBoot_ExtendsNotificationManager(t *testing.T) {
	provider := &ServiceProvider{}
	app := mocksfoundation.NewApplication(t)
	expectPublishes(app)
	mgr := mocksnotification.NewManager(t)
	config := mocksconfig.NewConfig(t)

	app.EXPECT().MakeNotification().Return(mgr).Once()
	app.EXPECT().MakeHttp().Return(mocksclient.NewFactory(t)).Once()
	app.EXPECT().MakeConfig().Return(config).Once()
	config.EXPECT().GetString("slack.token").Return("xoxb-test").Once()
	mgr.EXPECT().Extend(mock.AnythingOfType("*slack.Channel")).Once()

	provider.Boot(app)
}
