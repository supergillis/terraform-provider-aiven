package usersettings_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	avngen "github.com/aiven/go-client-codegen"
	"github.com/aiven/go-client-codegen/handler/clickhouse"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/require"

	acc "github.com/aiven/terraform-provider-aiven/internal/acctest"
	"github.com/aiven/terraform-provider-aiven/internal/clickhousesql"
	"github.com/aiven/terraform-provider-aiven/internal/schemautil"
)

const resourceName = "aiven_clickhouse_user_settings.foo"

func TestAccAivenClickHouseUserSettings(t *testing.T) {
	projectName := acc.ProjectName()
	serviceName := acc.RandName("clickhouse")
	serviceIsReady := acc.CreateTestService(
		t,
		projectName,
		serviceName,
		acc.WithServiceType("clickhouse"),
		acc.WithPlan("startup-8"),
		acc.WithCloud("google-europe-west1"),
	)

	client, err := acc.GetTestGenAivenClient()
	require.NoError(t, err)

	userName := acc.RandName("user")
	expectEmptyPlanAfterRefresh := resource.ConfigPlanChecks{
		PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
	}
	initialConfig := testAccClickHouseUserSettingsInitial(projectName, serviceName, userName)
	updatedConfig := testAccClickHouseUserSettingsUpdated(projectName, serviceName, userName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acc.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acc.TestProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckClickHouseUserSettingsDestroy(t, client),
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					require.NoError(t, <-serviceIsReady)
				},
				Config:           initialConfig,
				ConfigPlanChecks: expectEmptyPlanAfterRefresh,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "id", fmt.Sprintf("%s/%s/%s", projectName, serviceName, userName)),
					resource.TestCheckResourceAttr(resourceName, "settings.%", "2"),
					resource.TestCheckResourceAttr(resourceName, "settings.max_execution_time", "60"),
					resource.TestCheckResourceAttr(resourceName, "settings.max_result_rows", "1000000"),
				),
			},
			{
				// Import has no config to keep a spelling from, so it reports the server value as is.
				Config:            initialConfig,
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Leaving out max_result_rows proves a setting missing from the config is dropped.
				Config:           updatedConfig,
				ConfigPlanChecks: expectEmptyPlanAfterRefresh,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "settings.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "settings.max_execution_time", "30"),
					resource.TestCheckNoResourceAttr(resourceName, "settings.max_result_rows"),
					testAccCheckClickHouseUserSettingNames(t, client, projectName, serviceName, userName, []string{"max_execution_time"}),
				),
			},
			{
				// Deleting the user outside Terraform removes the settings from state, so the plan recreates both.
				Config: updatedConfig,
				PreConfig: func() {
					users, err := client.ServiceClickHouseUserList(t.Context(), projectName, serviceName)
					require.NoError(t, err)
					index := slices.IndexFunc(users, func(user clickhouse.UserOut) bool { return user.Name == userName })
					require.NotEqual(t, -1, index)
					require.NoError(t, client.ServiceClickHouseUserDelete(t.Context(), projectName, serviceName, users[index].Uuid))
				},
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config:           updatedConfig,
				ConfigPlanChecks: expectEmptyPlanAfterRefresh,
				Check:            testAccCheckClickHouseUserSettingNames(t, client, projectName, serviceName, userName, []string{"max_execution_time"}),
			},
		},
	})
}

func testAccClickHouseUserSettingsInitial(project, serviceName, userName string) string {
	return fmt.Sprintf(`
resource "aiven_clickhouse_user" "foo" {
  project      = %[1]q
  service_name = %[2]q
  username     = %[3]q
}

resource "aiven_clickhouse_user_settings" "foo" {
  project      = aiven_clickhouse_user.foo.project
  service_name = aiven_clickhouse_user.foo.service_name
  username     = aiven_clickhouse_user.foo.username

  settings = {
    max_execution_time = "60"
    max_result_rows    = "1000000"
  }
}
`, project, serviceName, userName)
}

func testAccClickHouseUserSettingsUpdated(project, serviceName, userName string) string {
	return fmt.Sprintf(`
resource "aiven_clickhouse_user" "foo" {
  project      = %[1]q
  service_name = %[2]q
  username     = %[3]q
}

resource "aiven_clickhouse_user_settings" "foo" {
  project      = aiven_clickhouse_user.foo.project
  service_name = aiven_clickhouse_user.foo.service_name
  username     = aiven_clickhouse_user.foo.username

  settings = {
    max_execution_time = "30"
  }
}
`, project, serviceName, userName)
}

// testAccCheckClickHouseUserSettingNames checks the user's direct settings on the server,
// independent of what the provider reports.
func testAccCheckClickHouseUserSettingNames(t *testing.T, client avngen.Client, project, serviceName, userName string, expected []string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		names, err := serverSettingNames(t.Context(), client, project, serviceName, userName)
		if err != nil {
			return err
		}
		if !slices.Equal(names, expected) {
			return fmt.Errorf("expected settings %v on the server, got %v", expected, names)
		}
		return nil
	}
}

func testAccCheckClickHouseUserSettingsDestroy(t *testing.T, client avngen.Client) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		for _, resourceState := range state.RootModule().Resources {
			if resourceState.Type != "aiven_clickhouse_user_settings" {
				continue
			}
			project, serviceName, userName, err := schemautil.SplitResourceID3(resourceState.Primary.ID)
			if err != nil {
				return err
			}
			names, err := serverSettingNames(t.Context(), client, project, serviceName, userName)
			if err != nil && !avngen.IsNotFound(err) {
				return err
			}
			if len(names) > 0 {
				return fmt.Errorf("clickhouse user %q still has settings %v", userName, names)
			}
		}
		return nil
	}
}

func serverSettingNames(ctx context.Context, client avngen.Client, project, serviceName, userName string) ([]string, error) {
	response, err := client.ServiceClickHouseQuery(ctx, project, serviceName, &clickhouse.ServiceClickHouseQueryIn{
		Database: clickhousesql.SystemDatabase,
		Query: "SELECT setting_name FROM system.settings_profile_elements WHERE user_name = " +
			clickhousesql.QuoteString(userName) + " AND setting_name IS NOT NULL ORDER BY setting_name",
	})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(response.Data))
	for _, row := range response.Data {
		names = append(names, row[0].(string))
	}
	return names, nil
}
