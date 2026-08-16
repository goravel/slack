package slack

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/goravel/framework/contracts/binding"
	mocksconfig "github.com/goravel/framework/mocks/config"
	mocksfoundation "github.com/goravel/framework/mocks/foundation"
	mocksnotification "github.com/goravel/framework/mocks/notification"
)

func TestServiceProviderRelationship(t *testing.T) {
	provider := &ServiceProvider{}
	rel := provider.Relationship()

	assert.Empty(t, rel.Bindings)
	assert.Contains(t, rel.Dependencies, binding.Notification)
	assert.Contains(t, rel.Dependencies, binding.Config)
	assert.NotContains(t, rel.Dependencies, binding.Http, "Http dropped — slack-go/slack manages its own client now")
	assert.Empty(t, rel.ProvideFor)
}

func TestServiceProviderRegister_IsNoOp(t *testing.T) {
	provider := &ServiceProvider{}
	app := mocksfoundation.NewApplication(t) // no calls expected

	assert.NotPanics(t, func() {
		provider.Register(app)
	})
}

func TestServiceProviderBoot_SkipsWhenNotificationFacadeNotSet(t *testing.T) {
	provider := &ServiceProvider{}
	app := mocksfoundation.NewApplication(t)
	app.EXPECT().MakeNotification().Return(nil).Once()

	assert.NotPanics(t, func() {
		provider.Boot(app)
	})
}

func TestServiceProviderBoot_SkipsWhenConfigFacadeNotSet(t *testing.T) {
	provider := &ServiceProvider{}
	app := mocksfoundation.NewApplication(t)
	mgr := mocksnotification.NewManager(t)
	app.EXPECT().MakeNotification().Return(mgr).Once()
	app.EXPECT().MakeConfig().Return(nil).Once()

	assert.NotPanics(t, func() {
		provider.Boot(app)
	})
}

func TestServiceProviderBoot_ExtendsNotificationManager(t *testing.T) {
	provider := &ServiceProvider{}
	app := mocksfoundation.NewApplication(t)
	mgr := mocksnotification.NewManager(t)
	config := mocksconfig.NewConfig(t)

	app.EXPECT().MakeNotification().Return(mgr).Once()
	app.EXPECT().MakeConfig().Return(config).Once()
	config.EXPECT().GetString("slack.token").Return("xoxb-test").Once()
	mgr.EXPECT().Extend(mock.AnythingOfType("*slack.Channel")).Once()

	provider.Boot(app)
}
