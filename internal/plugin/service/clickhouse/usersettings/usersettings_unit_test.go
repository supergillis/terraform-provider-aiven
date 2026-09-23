package usersettings

import (
	"context"
	"testing"

	avngen "github.com/aiven/go-client-codegen"
	"github.com/aiven/go-client-codegen/handler/clickhouse"
	"github.com/stretchr/testify/require"

	"github.com/aiven/terraform-provider-aiven/internal/plugin/adapter"
)

func expectUserList(client *avngen.MockClient, usernames ...string) {
	users := make([]clickhouse.UserOut, 0, len(usernames))
	for _, username := range usernames {
		users = append(users, clickhouse.UserOut{Name: username})
	}
	client.EXPECT().ServiceClickHouseUserList(context.Background(), "project", "clickhouse").Return(users, nil)
}

func TestCreateSettingsSetsID(t *testing.T) {
	t.Parallel()

	client := avngen.NewMockClient(t)
	values := settingsValues(map[string]any{"max_threads": "4"})
	delete(values, "id")
	data, err := adapter.NewResourceData(resourceSchemaInternal(), idFields(),
		adapter.WithTestPlan(values),
		adapter.WithTestConfig(values),
	)
	require.NoError(t, err)
	expectSettingsQuery(client).Return(&clickhouse.ServiceClickHouseQueryOut{}, nil)
	expectQuery(client, "ALTER USER `reader` ADD SETTINGS `max_threads` = '4'").
		Return(&clickhouse.ServiceClickHouseQueryOut{}, nil)

	require.NoError(t, createSettings(context.Background(), client, data))
	require.Equal(t, "project/clickhouse/reader", data.ID())
}

func TestReadSettingsMissingUserIsNotFound(t *testing.T) {
	t.Parallel()

	client := avngen.NewMockClient(t)
	expectUserList(client, "writer")

	err := readSettings(context.Background(), client, stateData(t, settingsValues(map[string]any{})))
	require.EqualError(t, err, `lookup user "reader": not found`)
	require.True(t, adapter.IsNotFound(err))
}

func TestReadSettingsSetsSettingsExactlyAsReported(t *testing.T) {
	t.Parallel()

	client := avngen.NewMockClient(t)
	expectUserList(client, "reader")
	data := stateData(t, settingsValues(map[string]any{"max_memory_usage": "1G"}))
	expectSettingsQuery(client).Return(&clickhouse.ServiceClickHouseQueryOut{Data: [][]any{
		{"max_memory_usage", "1000000000"},
	}}, nil)

	require.NoError(t, readSettings(context.Background(), client, data))
	require.Equal(t, map[string]any{"max_memory_usage": "1000000000"}, data.Get("settings"))
}

func TestReadSettingsImportReportsAllSettings(t *testing.T) {
	t.Parallel()

	client := avngen.NewMockClient(t)
	expectUserList(client, "reader")
	data := stateData(t, settingsValues(nil))
	expectSettingsQuery(client).Return(&clickhouse.ServiceClickHouseQueryOut{Data: [][]any{
		{"max_memory_usage", "1000000000"},
		{"readonly", "1"},
	}}, nil)

	require.NoError(t, readSettings(context.Background(), client, data))
	require.Equal(t, map[string]any{
		"max_memory_usage": "1000000000",
		"readonly":         "1",
	}, data.Get("settings"))
}

func TestDeleteSettingsDropsAll(t *testing.T) {
	t.Parallel()

	client := avngen.NewMockClient(t)
	expectSettingsQuery(client).Return(&clickhouse.ServiceClickHouseQueryOut{Data: [][]any{
		{"max_execution_time", "60"},
		{"max_result_rows", "1000000"},
	}}, nil)
	expectQuery(client, "ALTER USER `reader` DROP SETTINGS `max_execution_time`, `max_result_rows`").
		Return(&clickhouse.ServiceClickHouseQueryOut{}, nil)

	require.NoError(t, applySettings(context.Background(), client, stateData(t, settingsValues(nil)), nil))
}

func TestDeleteSettingsWithoutSettingsSendsNoDrop(t *testing.T) {
	t.Parallel()

	client := avngen.NewMockClient(t)
	expectSettingsQuery(client).Return(&clickhouse.ServiceClickHouseQueryOut{}, nil)

	require.NoError(t, applySettings(context.Background(), client, stateData(t, settingsValues(nil)), nil))
}
