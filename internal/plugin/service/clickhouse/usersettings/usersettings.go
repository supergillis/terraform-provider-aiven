package usersettings

import (
	"context"
	"fmt"

	avngen "github.com/aiven/go-client-codegen"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/aiven/terraform-provider-aiven/internal/plugin/adapter"
)

func NewResource() resource.Resource {
	return adapter.NewResource(adapter.ResourceOptions{
		TypeName:       typeName,
		IDFields:       idFields(),
		Schema:         resourceSchema,
		SchemaInternal: resourceSchemaInternal(),
		RemoveMissing:  true,
		Create:         createSettings,
		Read:           readSettings,
		Update: func(ctx context.Context, client avngen.Client, d adapter.ResourceData) error {
			return applySettings(ctx, client, d, desiredSettings(d))
		},
		Delete: func(ctx context.Context, client avngen.Client, d adapter.ResourceData) error {
			return applySettings(ctx, client, d, nil)
		},
	})
}

func createSettings(ctx context.Context, client avngen.Client, d adapter.ResourceData) error {
	if err := applySettings(ctx, client, d, desiredSettings(d)); err != nil {
		return err
	}
	return d.SetID(d.Get("project").(string), d.Get("service_name").(string), d.Get("username").(string))
}

// readSettings reports a missing user as not found, so RemoveMissing drops the resource.
// An empty settings query cannot tell a user without settings from a deleted user.
func readSettings(ctx context.Context, client avngen.Client, d adapter.ResourceData) error {
	project := d.Get("project").(string)
	serviceName := d.Get("service_name").(string)
	username := d.Get("username").(string)
	users, err := client.ServiceClickHouseUserList(ctx, project, serviceName)
	if err != nil {
		return err
	}
	_, err = adapter.FindOne(users, func(i int) bool { return users[i].Name == username })
	if err != nil {
		return fmt.Errorf("lookup user %q: %w", username, err)
	}

	current, err := querySettings(ctx, client, d)
	if err != nil {
		return err
	}
	if err := d.Set("settings", current); err != nil {
		return err
	}
	// Import sets only the ID fields, so Read fills in the id.
	return d.SetID(project, serviceName, username)
}
