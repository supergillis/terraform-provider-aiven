package user

import (
	"context"
	"testing"

	avngen "github.com/aiven/go-client-codegen"
	"github.com/aiven/go-client-codegen/handler/clickhouse"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/stretchr/testify/require"

	"github.com/aiven/terraform-provider-aiven/internal/plugin/adapter"
)

func TestResourceSchemaWithSettingsAllowsValueOmitted(t *testing.T) {
	t.Parallel()

	settingsAttribute, ok := resourceSchemaWithSettings(context.Background()).Attributes["settings"].(schema.MapNestedAttribute)
	require.True(t, ok)
	valueAttribute, ok := settingsAttribute.NestedObject.Attributes["value"].(schema.StringAttribute)
	require.True(t, ok)
	require.True(t, valueAttribute.Optional)
	require.False(t, valueAttribute.Required)
}

func TestModifySettingsStatement(t *testing.T) {
	t.Parallel()

	settings := map[string]userSetting{
		"max_execution_time": {
			Value: new("60"),
			Max:   new("60"),
		},
		"max_result_rows": {
			Value:       new("1000000"),
			Max:         new("1000000"),
			Writability: "CONST",
		},
	}

	require.Equal(t,
		"ALTER USER `reader` MODIFY SETTINGS `max_execution_time` = '60' MAX '60', "+
			"`max_result_rows` = '1000000' MAX '1000000' CONST",
		modifySettingsStatement("reader", settings),
	)
}

func TestDropSettingsStatementEscapesIdentifiers(t *testing.T) {
	t.Parallel()

	require.Equal(t,
		"ALTER USER `read\\`er` DROP SETTINGS `max\\`rows`, `timeout`",
		dropSettingsStatement("read`er", []string{"timeout", "max`rows"}),
	)
}

func TestModifySettingsStatementEscapesValues(t *testing.T) {
	t.Parallel()

	require.Equal(t,
		"ALTER USER `reader` MODIFY SETTINGS `custom_setting` = 'can\\'t \\\\ change' WRITABLE",
		modifySettingsStatement("reader", map[string]userSetting{
			"custom_setting": {Value: new("can't \\ change"), Writability: "WRITABLE"},
		}),
	)
}

func TestModifySettingsStatementWithoutValue(t *testing.T) {
	t.Parallel()

	require.Equal(t,
		"ALTER USER `reader` MODIFY SETTINGS `max_execution_time` MAX '60' CONST",
		modifySettingsStatement("reader", map[string]userSetting{
			"max_execution_time": {Max: new("60"), Writability: "CONST"},
		}),
	)
}

func TestSettingsFromQueryResponse(t *testing.T) {
	t.Parallel()

	response := &clickhouse.ServiceClickHouseQueryOut{
		Data: [][]any{
			{"max_execution_time", "60", nil, "60", "WRITABLE"},
			{"max_result_rows", "1000000", "10", "1000000", "CONST"},
			{"max_threads", nil, nil, "4", "CONST"},
		},
	}

	settings, err := settingsFromQueryResponse(response)
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"max_execution_time": map[string]any{
			"value":       "60",
			"max":         "60",
			"writability": "WRITABLE",
		},
		"max_result_rows": map[string]any{
			"value":       "1000000",
			"min":         "10",
			"max":         "1000000",
			"writability": "CONST",
		},
		"max_threads": map[string]any{
			"max":         "4",
			"writability": "CONST",
		},
	}, settings)
}

func TestReconcileConfiguredSettings(t *testing.T) {
	t.Parallel()

	const project = "project"
	const serviceName = "clickhouse"
	const username = "reader"

	ctx := context.Background()
	client := avngen.NewMockClient(t)
	settings := map[string]any{
		"max_execution_time": map[string]any{
			"value": "60",
			"max":   "60",
		},
		"max_result_rows": map[string]any{
			"value": "1000000",
			"max":   "1000000",
		},
	}
	values := map[string]any{
		"project":      project,
		"service_name": serviceName,
		"username":     username,
		"settings":     settings,
	}
	data, err := adapter.NewResourceData(ResourceOptions.SchemaInternal, idFields(),
		adapter.WithTestPlan(values),
		adapter.WithTestConfig(values),
	)
	require.NoError(t, err)

	client.EXPECT().ServiceClickHouseQuery(ctx, project, serviceName, &clickhouse.ServiceClickHouseQueryIn{
		Database: clickHouseSystemDatabase,
		Query: "SELECT setting_name, value, min, max, writability FROM system.settings_profile_elements " +
			"WHERE user_name = 'reader' AND setting_name IS NOT NULL ORDER BY setting_name",
	}).Return(&clickhouse.ServiceClickHouseQueryOut{Data: [][]any{
		{"max_execution_time", "60", nil, "60", "WRITABLE"},
		{"max_result_rows", "500000", nil, "500000", "WRITABLE"},
		{"obsolete_setting", "1", nil, nil, "WRITABLE"},
	}}, nil)
	client.EXPECT().ServiceClickHouseQuery(ctx, project, serviceName, &clickhouse.ServiceClickHouseQueryIn{
		Database: clickHouseSystemDatabase,
		Query:    "ALTER USER `reader` DROP SETTINGS `obsolete_setting`",
	}).Return(&clickhouse.ServiceClickHouseQueryOut{}, nil)
	client.EXPECT().ServiceClickHouseQuery(ctx, project, serviceName, &clickhouse.ServiceClickHouseQueryIn{
		Database: clickHouseSystemDatabase,
		Query:    "ALTER USER `reader` MODIFY SETTINGS `max_result_rows` = '1000000' MAX '1000000' WRITABLE",
	}).Return(&clickhouse.ServiceClickHouseQueryOut{}, nil)

	require.NoError(t, reconcileConfiguredSettings(ctx, client, data))
}

func TestReconcileConfiguredSettingsLeavesOmittedSettingsUnmanaged(t *testing.T) {
	t.Parallel()

	values := map[string]any{
		"project":      "project",
		"service_name": "clickhouse",
		"username":     "reader",
	}
	data, err := adapter.NewResourceData(ResourceOptions.SchemaInternal, idFields(),
		adapter.WithTestPlan(values),
		adapter.WithTestConfig(values),
	)
	require.NoError(t, err)

	require.NoError(t, reconcileConfiguredSettings(context.Background(), avngen.NewMockClient(t), data))
}
