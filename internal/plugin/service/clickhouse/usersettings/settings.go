package usersettings

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	avngen "github.com/aiven/go-client-codegen"
	"github.com/aiven/go-client-codegen/handler/clickhouse"

	"github.com/aiven/terraform-provider-aiven/internal/clickhousesql"
	"github.com/aiven/terraform-provider-aiven/internal/plugin/adapter"
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

// applySettings makes the user's direct settings match desired, dropping the rest.
// ADD SETTINGS replaces a whole element and is idempotent, so every desired setting is sent
// and only settings missing from desired are dropped. A nil desired drops everything.
func applySettings(ctx context.Context, client avngen.Client, d adapter.ResourceData, desired map[string]string) error {
	current, err := querySettings(ctx, client, d)
	if err != nil {
		return err
	}

	username := d.Get("username").(string)
	var removed []string
	for name := range current {
		if _, ok := desired[name]; !ok {
			removed = append(removed, name)
		}
	}
	if len(removed) > 0 {
		if _, err := runQuery(ctx, client, d, dropSettingsStatement(username, removed)); err != nil {
			return err
		}
	}

	if len(desired) == 0 {
		return nil
	}
	_, err = runQuery(ctx, client, d, addSettingsStatement(username, desired))
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
		Database: clickhousesql.SystemDatabase,
		Query:    statement,
	})
}

// Column positions in settingsQuery.
const (
	columnName = iota
	columnValue
	columnCount
)

func settingsQuery(username string) string {
	return fmt.Sprintf(`
SELECT
    setting_name,
    value
FROM system.settings_profile_elements
WHERE user_name = %s
    AND setting_name IS NOT NULL
ORDER BY setting_name`, clickhousesql.QuoteString(username))
}

func serverSettingsFromRows(rows [][]any) (map[string]string, error) {
	settings := make(map[string]string, len(rows))
	for index, row := range rows {
		if len(row) != columnCount {
			return nil, fmt.Errorf("settings row %d has %d columns, expected %d", index, len(row), columnCount)
		}
		name, ok := row[columnName].(string)
		if !ok {
			return nil, fmt.Errorf("settings row %d has setting name %v, expected a string", index, row[columnName])
		}
		if _, ok := settings[name]; ok {
			return nil, fmt.Errorf("setting %q is set more than once for the user, remove the duplicates in ClickHouse", name)
		}

		value, _ := row[columnValue].(string)
		settings[name] = value
	}
	return settings, nil
}

func addSettingsStatement(username string, settings map[string]string) string {
	clauses := make([]string, 0, len(settings))
	for _, name := range slices.Sorted(maps.Keys(settings)) {
		clauses = append(clauses, fmt.Sprintf("%s = %s", clickhousesql.QuoteIdentifier(name), clickhousesql.QuoteString(settings[name])))
	}
	return fmt.Sprintf("ALTER USER %s ADD SETTINGS %s", clickhousesql.QuoteIdentifier(username), strings.Join(clauses, ", "))
}

func dropSettingsStatement(username string, names []string) string {
	quoted := make([]string, 0, len(names))
	for _, name := range slices.Sorted(slices.Values(names)) {
		quoted = append(quoted, clickhousesql.QuoteIdentifier(name))
	}
	return fmt.Sprintf("ALTER USER %s DROP SETTINGS %s", clickhousesql.QuoteIdentifier(username), strings.Join(quoted, ", "))
}
