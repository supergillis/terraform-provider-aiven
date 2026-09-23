package usersettings

import (
	"context"
	"testing"

	avngen "github.com/aiven/go-client-codegen"
	"github.com/aiven/go-client-codegen/handler/clickhouse"
	"github.com/stretchr/testify/require"

	"github.com/aiven/terraform-provider-aiven/internal/plugin/adapter"
)

func expectQuery(client *avngen.MockClient, statement string) *avngen.MockClient_ServiceClickHouseQuery_Call {
	return client.EXPECT().ServiceClickHouseQuery(context.Background(), "project", "clickhouse", &clickhouse.ServiceClickHouseQueryIn{
		Database: "system",
		Query:    statement,
	})
}

func expectSettingsQuery(client *avngen.MockClient) *avngen.MockClient_ServiceClickHouseQuery_Call {
	return expectQuery(client, settingsQuery("reader"))
}

func settingsValues(settings map[string]any) map[string]any {
	values := map[string]any{
		"id":           "project/clickhouse/reader",
		"project":      "project",
		"service_name": "clickhouse",
		"username":     "reader",
	}
	if settings != nil {
		values["settings"] = settings
	}
	return values
}

func settingsData(t *testing.T, settings map[string]any) adapter.ResourceData {
	t.Helper()

	values := settingsValues(settings)
	data, err := adapter.NewResourceData(resourceSchemaInternal(), idFields(),
		adapter.WithTestPlan(values),
		adapter.WithTestConfig(values),
	)
	require.NoError(t, err)
	return data
}

func stateData(t *testing.T, values map[string]any) adapter.ResourceData {
	t.Helper()

	data, err := adapter.NewResourceData(resourceSchemaInternal(), idFields(), adapter.WithTestState(values))
	require.NoError(t, err)
	return data
}

func TestAlterSettingsStatementSortsKeysAndQuotesValues(t *testing.T) {
	t.Parallel()

	settings := map[string]string{
		"max_result_rows":    "1000000",
		"max_execution_time": "60",
		"format_schema":      "it's",
	}

	require.Equal(t,
		"ALTER USER IF EXISTS `O\\`Neil` DROP ALL SETTINGS ADD SETTINGS `format_schema` = 'it\\'s', `max_execution_time` = '60', `max_result_rows` = '1000000'",
		alterSettingsStatement("O`Neil", settings),
	)
}

func TestAlterSettingsStatementEmptyDropsAll(t *testing.T) {
	t.Parallel()

	require.Equal(t, "ALTER USER IF EXISTS `reader` DROP ALL SETTINGS", alterSettingsStatement("reader", nil))
}

func TestServerSettingsFromRows(t *testing.T) {
	t.Parallel()

	settings, err := serverSettingsFromRows([][]any{
		{"max_result_rows", "1000000"},
		{"SQL_custom", "'abc'"},
	})
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"max_result_rows": "1000000",
		"SQL_custom":      "'abc'",
	}, settings)
}

func TestServerSettingsFromRowsRejectsMissingColumn(t *testing.T) {
	t.Parallel()

	_, err := serverSettingsFromRows([][]any{{"max_result_rows"}})
	require.EqualError(t, err, "settings row 0 has 1 columns, expected 2")
}

func TestApplySettingsSendsSingleAlterStatement(t *testing.T) {
	t.Parallel()

	client := avngen.NewMockClient(t)
	data := settingsData(t, map[string]any{
		"max_execution_time": "60",
		"max_result_rows":    "1000000",
	})
	expectQuery(client, "ALTER USER IF EXISTS `reader` DROP ALL SETTINGS ADD SETTINGS `max_execution_time` = '60', `max_result_rows` = '1000000'").
		Return(&clickhouse.ServiceClickHouseQueryOut{}, nil)

	require.NoError(t, applySettings(context.Background(), client, data, desiredSettings(data)))
}
