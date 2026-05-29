package notifications_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/t0mer/speedtest-exporter/internal/database"
	"github.com/t0mer/speedtest-exporter/internal/notifications"
)

func openTestStore(t *testing.T) *notifications.Store {
	t.Helper()
	db, err := database.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return notifications.NewStore(db.SQL(), key)
}

func makeShoutrrr(url string) json.RawMessage {
	b, _ := json.Marshal(notifications.ShoutrrrConfig{URL: url})
	return b
}

func TestStoreListEmpty(t *testing.T) {
	store := openTestStore(t)
	channels, err := store.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, channels)
}

func TestStoreSaveAndGet(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	ch := &notifications.Channel{
		Name:            "My Slack",
		Provider:        notifications.ProviderShoutrrr,
		Config:          makeShoutrrr("slack://token@channel"),
		Enabled:         true,
		NotifyOnSuccess: true,
		NotifyOnFailure: true,
	}
	require.NoError(t, store.Save(ctx, ch))
	assert.Positive(t, ch.ID)

	got, err := store.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "My Slack", got.Name)
	assert.Equal(t, notifications.ProviderShoutrrr, got.Provider)

	var cfg notifications.ShoutrrrConfig
	require.NoError(t, json.Unmarshal(got.Config, &cfg))
	assert.Equal(t, "slack://token@channel", cfg.URL)
}

func TestStoreUpdate(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	ch := &notifications.Channel{
		Name: "old", Provider: notifications.ProviderShoutrrr,
		Config: makeShoutrrr("slack://a@b"), Enabled: true,
		NotifyOnSuccess: false, NotifyOnFailure: true,
	}
	require.NoError(t, store.Save(ctx, ch))
	ch.Name = "updated"
	ch.NotifyOnSuccess = true
	require.NoError(t, store.Update(ctx, ch))

	got, _ := store.Get(ctx, ch.ID)
	assert.Equal(t, "updated", got.Name)
	assert.True(t, got.NotifyOnSuccess)
}

func TestStoreDelete(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	ch := &notifications.Channel{
		Name: "to delete", Provider: notifications.ProviderShoutrrr,
		Config: makeShoutrrr("slack://x@y"), Enabled: true,
	}
	require.NoError(t, store.Save(ctx, ch))
	require.NoError(t, store.Delete(ctx, ch.ID))

	got, err := store.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestStoreMaskConfig(t *testing.T) {
	ch := &notifications.Channel{
		ID: 1, Name: "Slack", Provider: notifications.ProviderShoutrrr,
		Config: makeShoutrrr("slack://secret-token@my-channel"),
	}
	view := ch.ToView()
	var cfg notifications.ShoutrrrConfig
	require.NoError(t, json.Unmarshal(view.Config, &cfg))
	assert.Equal(t, "slack://***", cfg.URL)
	assert.NotContains(t, string(view.Config), "secret-token")
}

func TestDeleteAll(t *testing.T) {
	db, err := database.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	key := make([]byte, 32)
	s := notifications.NewStore(db.SQL(), key)
	ctx := context.Background()

	ch1 := &notifications.Channel{
		Name:     "Alpha",
		Provider: notifications.ProviderShoutrrr,
		Config:   json.RawMessage(`{"url":"slack://t@c"}`),
		Enabled:  true,
	}
	ch2 := &notifications.Channel{
		Name:     "Beta",
		Provider: notifications.ProviderShoutrrr,
		Config:   json.RawMessage(`{"url":"discord://t@c"}`),
		Enabled:  true,
	}
	require.NoError(t, s.Save(ctx, ch1))
	require.NoError(t, s.Save(ctx, ch2))

	list, err := s.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	require.NoError(t, s.DeleteAll(ctx))

	list, err = s.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)
}
