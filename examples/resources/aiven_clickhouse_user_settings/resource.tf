resource "aiven_clickhouse_user_settings" "example" {
  project      = "my-project" // Force new
  service_name = "my-clickhouse" // Force new
  username     = "alice" // Force new
  settings = {
    max_execution_time = "60"
  }
}
