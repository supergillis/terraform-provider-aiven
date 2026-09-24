package usersettings

import (
	"context"
	"testing"

	avngen "github.com/aiven/go-client-codegen"
	"github.com/aiven/go-client-codegen/handler/clickhouse"
	"github.com/stretchr/testify/require"

	"github.com/aiven/terraform-provider-aiven/internal/clickhousesql"
	"github.com/aiven/terraform-provider-aiven/internal/plugin/adapter"
)

func expectQuery(client *avngen.MockClient, statement string) *avngen.MockClient_ServiceClickHouseQuery_Call {
	return client.EXPECT().ServiceClickHouseQuery(context.Background(), "project", "clickhouse", &clickhouse.ServiceClickHouseQueryIn{
		Database: clickhousesql.SystemDatabase,
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

func TestAddSettingsStatement(t *testing.T) {
	t.Parallel()

	settings := map[string]string{
		"max_result_rows":    "1000000",
		"max_execution_time": "60",
		"format_schema":      "it's",
	}

	require.Equal(t,
		"ALTER USER `O\\`Neil` ADD SETTINGS `format_schema` = 'it\\'s', `max_execution_time` = '60', `max_result_rows` = '1000000'",
		addSettingsStatement("O`Neil", settings),
	)
}

func TestDropSettingsStatement(t *testing.T) {
	t.Parallel()

	require.Equal(t,
		"ALTER USER `reader` DROP SETTINGS `max_result_rows`, `odd\\`name`",
		dropSettingsStatement("reader", []string{"odd`name", "max_result_rows"}),
	)
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

func TestServerSettingsFromRowsRejectsInvalidRows(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		rows    [][]any
		message string
	}{
		"missing column": {
			rows:    [][]any{{"max_result_rows"}},
			message: "settings row 0 has 1 columns, expected 2",
		},
		"duplicate": {
			rows: [][]any{
				{"max_result_rows", "1"},
				{"max_result_rows", "2"},
			},
			message: `setting "max_result_rows" is set more than once for the user, remove the duplicates in ClickHouse`,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			_, err := serverSettingsFromRows(testCase.rows)
			require.EqualError(t, err, testCase.message)
		})
	}
}

func TestApplySettingsDropsRemovedAndAddsAllDesired(t *testing.T) {
	t.Parallel()

	client := avngen.NewMockClient(t)
	data := settingsData(t, map[string]any{
		"max_execution_time": "60",
		"max_result_rows":    "1000000",
	})
	expectSettingsQuery(client).Return(&clickhouse.ServiceClickHouseQueryOut{Data: [][]any{
		{"max_execution_time", "60"},
		{"obsolete_setting", "1"},
	}}, nil)
	expectQuery(client, "ALTER USER `reader` DROP SETTINGS `obsolete_setting`").
		Return(&clickhouse.ServiceClickHouseQueryOut{}, nil)
	expectQuery(client, "ALTER USER `reader` ADD SETTINGS `max_execution_time` = '60', `max_result_rows` = '1000000'").
		Return(&clickhouse.ServiceClickHouseQueryOut{}, nil)

	require.NoError(t, applySettings(context.Background(), client, data, desiredSettings(data)))
}

func TestApplySettingsEmptyMapDropsAll(t *testing.T) {
	t.Parallel()

	client := avngen.NewMockClient(t)
	data := settingsData(t, map[string]any{})
	expectSettingsQuery(client).Return(&clickhouse.ServiceClickHouseQueryOut{Data: [][]any{
		{"max_execution_time", "60"},
		{"max_result_rows", "1000000"},
	}}, nil)
	expectQuery(client, "ALTER USER `reader` DROP SETTINGS `max_execution_time`, `max_result_rows`").
		Return(&clickhouse.ServiceClickHouseQueryOut{}, nil)

	require.NoError(t, applySettings(context.Background(), client, data, desiredSettings(data)))
}
