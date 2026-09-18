package user

import (
	"context"

	avngen "github.com/aiven/go-client-codegen"

	"github.com/aiven/terraform-provider-aiven/internal/plugin/adapter"
)

func init() {
	ResourceOptions.Create = createUser
	ResourceOptions.Read = readUser
	ResourceOptions.Update = updateUser
	ResourceOptions.Schema = resourceSchemaWithSettings
	ResourceOptions.SchemaInternal.Properties["settings"] = settingsSchemaInternal()
}

func createUser(ctx context.Context, client avngen.Client, d adapter.ResourceData) error {
	if err := createView(ctx, client, d); err != nil {
		return err
	}

	return reconcileConfiguredSettings(ctx, client, d)
}

func readUser(ctx context.Context, client avngen.Client, d adapter.ResourceData) error {
	if err := readView(ctx, client, d); err != nil {
		return err
	}

	if _, ok := d.GetOk("settings"); !ok {
		return nil
	}

	settings, err := readSettings(ctx, client, d)
	if err != nil {
		return err
	}
	return d.Set("settings", settings)
}

func updateUser(ctx context.Context, client avngen.Client, d adapter.ResourceData) error {
	if d.HasChange("password") || d.HasChange("password_wo_version") {
		if err := updateView(ctx, client, d); err != nil {
			return err
		}
	}

	if !d.HasChange("settings") {
		return nil
	}
	return reconcileConfiguredSettings(ctx, client, d)
}

func expandModifier(_ context.Context, _ avngen.Client) adapter.MapModifier {
	return func(d adapter.ResourceData, dto map[string]any) error {
		if v, ok := d.GetOk("password_wo"); ok {
			// Sets Write-only password to the password field in the API request
			dto["password"] = v
		}
		return nil
	}
}

func flattenModifier(_ context.Context, _ avngen.Client) adapter.MapModifier {
	return func(d adapter.ResourceData, dto map[string]any) error {
		if _, ok := d.Schema().Properties["password_wo_version"]; ok && d.Get("password_wo_version").(int) != 0 {
			// Clear password from state when using write-only password.
			delete(dto, "password")
		}

		return nil
	}
}
