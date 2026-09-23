package usersettings_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/aiven/go-client-codegen/handler/clickhouse"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/stretchr/testify/require"

	acc "github.com/aiven/terraform-provider-aiven/internal/acctest"
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
