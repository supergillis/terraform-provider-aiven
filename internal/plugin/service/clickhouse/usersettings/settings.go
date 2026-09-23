package usersettings

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	avngen "github.com/aiven/go-client-codegen"
	"github.com/aiven/go-client-codegen/handler/clickhouse"

	"github.com/aiven/terraform-provider-aiven/internal/plugin/adapter"
	sdkclickhouse "github.com/aiven/terraform-provider-aiven/internal/sdkprovider/service/clickhouse"
)

// desiredSettings returns the settings from the plan, or from the state during refresh.
func desiredSettings(d adapter.ResourceData) map[string]string {
	rawSettings, _ := d.Get("settings").(map[string]any)
	settings := make(map[string]string, len(rawSettings))
	for name, value := range rawSettings {
		text, _ := value.(string)
		settings[name] = text
	}
	return settings
}

// applySettings makes the user's direct settings match desired in a single statement.
// A nil or empty desired drops every direct setting.
func applySettings(ctx context.Context, client avngen.Client, d adapter.ResourceData, desired map[string]string) error {
	username := d.Get("username").(string)
	_, err := runQuery(ctx, client, d, alterSettingsStatement(username, desired))
	return err
}

func querySettings(ctx context.Context, client avngen.Client, d adapter.ResourceData) (map[string]string, error) {
	response, err := runQuery(ctx, client, d, settingsQuery(d.Get("username").(string)))
	if err != nil {
		return nil, err
	}
	return serverSettingsFromRows(response.Data)
}

func runQuery(ctx context.Context, client avngen.Client, d adapter.ResourceData, statement string) (*clickhouse.ServiceClickHouseQueryOut, error) {
	return client.ServiceClickHouseQuery(ctx, d.Get("project").(string), d.Get("service_name").(string), &clickhouse.ServiceClickHouseQueryIn{
		Database: "system",
		Query:    statement,
	})
}

func settingsQuery(username string) string {
	return fmt.Sprintf(`
SELECT
    setting_name,
    value
FROM system.settings_profile_elements
WHERE user_name = %s
    AND setting_name IS NOT NULL
    AND value IS NOT NULL
ORDER BY setting_name`, sdkclickhouse.QuoteString(username))
}

func serverSettingsFromRows(rows [][]any) (map[string]string, error) {
	settings := make(map[string]string, len(rows))
	for index, row := range rows {
		if len(row) != 2 {
			return nil, fmt.Errorf("settings row %d has %d columns, expected 2", index, len(row))
		}
		name, ok := row[0].(string)
		if !ok {
			return nil, fmt.Errorf("settings row %d has setting name %v, expected a string", index, row[0])
		}
		value, _ := row[1].(string)
		settings[name] = value
	}
	return settings, nil
}

// alterSettingsStatement drops every direct setting, then re-adds the desired ones, so a
// setting missing from desired never lingers.
func alterSettingsStatement(username string, settings map[string]string) string {
	statement := fmt.Sprintf("ALTER USER IF EXISTS %s DROP ALL SETTINGS", sdkclickhouse.Escape(username))
	if len(settings) == 0 {
		return statement
	}

	clauses := make([]string, 0, len(settings))
	for _, name := range slices.Sorted(maps.Keys(settings)) {
		clauses = append(clauses, fmt.Sprintf("%s = %s", sdkclickhouse.Escape(name), sdkclickhouse.QuoteString(settings[name])))
	}
	return statement + " ADD SETTINGS " + strings.Join(clauses, ", ")
}
