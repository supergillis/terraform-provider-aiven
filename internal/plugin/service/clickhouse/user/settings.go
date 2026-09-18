package user

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	avngen "github.com/aiven/go-client-codegen"
	"github.com/aiven/go-client-codegen/handler/clickhouse"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	"github.com/aiven/terraform-provider-aiven/internal/clickhousesql"
	"github.com/aiven/terraform-provider-aiven/internal/plugin/adapter"
)

const clickHouseSystemDatabase = "system"

var settingWritabilityValues = []string{"WRITABLE", "CONST", "CHANGEABLE_IN_READONLY"}

type userSetting struct {
	Value       *string
	Min         *string
	Max         *string
	Writability string
}

func resourceSchemaWithSettings(ctx context.Context) schema.Schema {
	result := resourceSchema(ctx)
	result.Attributes["settings"] = schema.MapNestedAttribute{
		MarkdownDescription: "Direct ClickHouse settings for the user. Omit this attribute to leave direct settings unmanaged. Set it to an empty map to remove all direct settings. The API token requires the `service:data:write` permission.",
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"value": schema.StringAttribute{
					MarkdownDescription: "Setting value.",
					Optional:            true,
				},
				"min": schema.StringAttribute{
					MarkdownDescription: "Minimum allowed value.",
					Optional:            true,
				},
				"max": schema.StringAttribute{
					MarkdownDescription: "Maximum allowed value.",
					Optional:            true,
				},
				"writability": schema.StringAttribute{
					Computed:            true,
					MarkdownDescription: "Whether the setting can be changed by the user. The possible values are `WRITABLE`, `CONST`, and `CHANGEABLE_IN_READONLY`.",
					Optional:            true,
					Validators:          []validator.String{stringvalidator.OneOf(settingWritabilityValues...)},
				},
			},
		},
		Optional: true,
	}
	return result
}

func settingsSchemaInternal() *adapter.Schema {
	return &adapter.Schema{
		Type: adapter.SchemaTypeMap,
		Items: &adapter.Schema{
			Type: adapter.SchemaTypeObject,
			Properties: map[string]*adapter.Schema{
				"value":       {Type: adapter.SchemaTypeString},
				"min":         {Type: adapter.SchemaTypeString},
				"max":         {Type: adapter.SchemaTypeString},
				"writability": {Type: adapter.SchemaTypeString, Computed: true},
			},
		},
	}
}

func reconcileConfiguredSettings(ctx context.Context, client avngen.Client, d adapter.ResourceData) error {
	rawSettings, managed := d.GetConfigOk("settings")
	if !managed {
		return nil
	}

	desired, err := userSettingsFromValue(rawSettings)
	if err != nil {
		return err
	}
	current, err := querySettings(ctx, client, d)
	if err != nil {
		return err
	}

	removed, changed := settingsChanges(current, desired)
	if len(removed) > 0 {
		if err := executeSettingsQuery(ctx, client, d, dropSettingsStatement(d.Get("username").(string), removed)); err != nil {
			return err
		}
	}

	if len(changed) == 0 {
		return nil
	}
	return executeSettingsQuery(ctx, client, d, modifySettingsStatement(d.Get("username").(string), changed))
}

func readSettings(ctx context.Context, client avngen.Client, d adapter.ResourceData) (map[string]any, error) {
	settings, err := querySettings(ctx, client, d)
	if err != nil {
		return nil, err
	}
	return settingsState(settings), nil
}

func querySettings(ctx context.Context, client avngen.Client, d adapter.ResourceData) (map[string]userSetting, error) {
	query := fmt.Sprintf(
		"SELECT setting_name, value, min, max, writability "+
			"FROM system.settings_profile_elements "+
			"WHERE user_name = %s AND setting_name IS NOT NULL ORDER BY setting_name",
		clickhousesql.QuoteString(d.Get("username").(string)),
	)
	response, err := client.ServiceClickHouseQuery(ctx, d.Get("project").(string), d.Get("service_name").(string), &clickhouse.ServiceClickHouseQueryIn{
		Database: clickHouseSystemDatabase,
		Query:    query,
	})
	if err != nil {
		return nil, err
	}
	return userSettingsFromQueryResponse(response)
}

func executeSettingsQuery(ctx context.Context, client avngen.Client, d adapter.ResourceData, query string) error {
	_, err := client.ServiceClickHouseQuery(ctx, d.Get("project").(string), d.Get("service_name").(string), &clickhouse.ServiceClickHouseQueryIn{
		Database: clickHouseSystemDatabase,
		Query:    query,
	})
	return err
}

func userSettingsFromValue(value any) (map[string]userSetting, error) {
	rawSettings, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected settings to be map[string]any, got %T", value)
	}

	settings := make(map[string]userSetting, len(rawSettings))
	for name, rawSetting := range rawSettings {
		fields, ok := rawSetting.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected setting %q to be map[string]any, got %T", name, rawSetting)
		}
		setting, err := userSettingFromFields(fields)
		if err != nil {
			return nil, fmt.Errorf("invalid setting %q: %w", name, err)
		}
		settings[name] = setting
	}
	return settings, nil
}

func userSettingFromFields(fields map[string]any) (userSetting, error) {
	value, err := optionalString(fields, "value")
	if err != nil {
		return userSetting{}, err
	}

	writability := "WRITABLE"
	if configuredWritability, ok := fields["writability"].(string); ok {
		writability = configuredWritability
	}
	if !slices.Contains(settingWritabilityValues, writability) {
		return userSetting{}, fmt.Errorf("unsupported writability %q", writability)
	}

	minimum, err := optionalString(fields, "min")
	if err != nil {
		return userSetting{}, err
	}
	maximum, err := optionalString(fields, "max")
	if err != nil {
		return userSetting{}, err
	}
	return userSetting{Value: value, Min: minimum, Max: maximum, Writability: writability}, nil
}

func optionalString(fields map[string]any, name string) (*string, error) {
	value, exists := fields[name]
	if !exists || value == nil {
		return nil, nil
	}
	text, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("%s must be a string", name)
	}
	return &text, nil
}

func userSettingsFromQueryResponse(response *clickhouse.ServiceClickHouseQueryOut) (map[string]userSetting, error) {
	settings := make(map[string]userSetting, len(response.Data))
	for rowIndex, row := range response.Data {
		if len(row) != 5 {
			return nil, fmt.Errorf("expected 5 columns in settings row %d, got %d", rowIndex, len(row))
		}
		name, ok := row[0].(string)
		if !ok {
			return nil, fmt.Errorf("expected setting name in row %d to be a string, got %T", rowIndex, row[0])
		}
		fields := map[string]any{"value": row[1], "min": row[2], "max": row[3], "writability": row[4]}
		setting, err := userSettingFromFields(fields)
		if err != nil {
			return nil, fmt.Errorf("invalid setting %q from query response: %w", name, err)
		}
		settings[name] = setting
	}
	return settings, nil
}

func settingsFromQueryResponse(response *clickhouse.ServiceClickHouseQueryOut) (map[string]any, error) {
	settings, err := userSettingsFromQueryResponse(response)
	if err != nil {
		return nil, err
	}
	return settingsState(settings), nil
}

func settingsState(settings map[string]userSetting) map[string]any {
	result := make(map[string]any, len(settings))
	for name, setting := range settings {
		fields := map[string]any{
			"writability": setting.Writability,
		}
		if setting.Value != nil {
			fields["value"] = *setting.Value
		}
		if setting.Min != nil {
			fields["min"] = *setting.Min
		}
		if setting.Max != nil {
			fields["max"] = *setting.Max
		}
		result[name] = fields
	}
	return result
}

func settingsChanges(current, desired map[string]userSetting) ([]string, map[string]userSetting) {
	removed := make([]string, 0)
	for name := range current {
		if _, exists := desired[name]; !exists {
			removed = append(removed, name)
		}
	}
	changed := make(map[string]userSetting)
	for name, desiredSetting := range desired {
		if currentSetting, exists := current[name]; !exists || !reflect.DeepEqual(currentSetting, desiredSetting) {
			changed[name] = desiredSetting
		}
	}
	sort.Strings(removed)
	return removed, changed
}

func modifySettingsStatement(username string, settings map[string]userSetting) string {
	names := make([]string, 0, len(settings))
	for name := range settings {
		names = append(names, name)
	}
	sort.Strings(names)

	clauses := make([]string, 0, len(names))
	for _, name := range names {
		setting := settings[name]
		clause := clickhousesql.QuoteIdentifier(name)
		if setting.Value != nil {
			clause += " = " + clickhousesql.QuoteString(*setting.Value)
		}
		if setting.Min != nil {
			clause += " MIN " + clickhousesql.QuoteString(*setting.Min)
		}
		if setting.Max != nil {
			clause += " MAX " + clickhousesql.QuoteString(*setting.Max)
		}
		if setting.Writability != "" {
			clause += " " + setting.Writability
		}
		clauses = append(clauses, clause)
	}
	return fmt.Sprintf("ALTER USER %s MODIFY SETTINGS %s", clickhousesql.QuoteIdentifier(username), strings.Join(clauses, ", "))
}

func dropSettingsStatement(username string, settingNames []string) string {
	names := slices.Clone(settingNames)
	sort.Strings(names)
	for index, name := range names {
		names[index] = clickhousesql.QuoteIdentifier(name)
	}
	return fmt.Sprintf("ALTER USER %s DROP SETTINGS %s", clickhousesql.QuoteIdentifier(username), strings.Join(names, ", "))
}
