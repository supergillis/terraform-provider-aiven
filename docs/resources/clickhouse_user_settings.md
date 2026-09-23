---
page_title: "aiven_clickhouse_user_settings Resource - terraform-provider-aiven"
subcategory: ""
description: |-
  Manages the direct ClickHouse settings of an Aiven for ClickHouse user. Use one resource per user: it owns all direct settings of the user, so settings missing from settings are removed. Deleting the resource removes all direct settings of the user. The API token requires the service:data:write permission. If this resource is missing (for example, after a service power off), it's removed from the state and a new create plan is generated.
---

# aiven_clickhouse_user_settings (Resource)

Manages the direct ClickHouse settings of an Aiven for ClickHouse user. Use one resource per user: it owns all direct settings of the user, so settings missing from `settings` are removed. Deleting the resource removes all direct settings of the user. The API token requires the `service:data:write` permission. If this resource is missing (for example, after a service power off), it's removed from the state and a new create plan is generated.

## Example Usage

```terraform
resource "aiven_clickhouse_user_settings" "example" {
  project      = "my-project" // Force new
  service_name = "my-clickhouse" // Force new
  username     = "alice" // Force new
  settings = {
    max_execution_time = "60"
  }
}
```

## Schema

### Required

- `project` (String) Project name. Changing this property forces recreation of the resource.
- `service_name` (String) Service name. Changing this property forces recreation of the resource.
- `settings` (Map of String) Direct ClickHouse settings for the user, keyed by setting name, with values written exactly as ClickHouse stores them in `system.settings_profile_elements`. Set `{}` to remove them all.
- `username` (String) Name of the ClickHouse user. Maximum length: `64`. Changing this property forces recreation of the resource.

### Optional

- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))

### Read-Only

- `id` (String) Resource ID composed as: `project/service_name/username`.

<a id="nestedblock--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- `create` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours).
- `delete` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours). Setting a timeout for a Delete operation is only applicable if changes are saved into state before the destroy operation occurs.
- `read` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours). Read operations occur during any refresh or planning operation when refresh is enabled.
- `update` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours).

## Import

Import is supported using the following syntax:

```shell
terraform import aiven_clickhouse_user_settings.example PROJECT/SERVICE_NAME/USERNAME
```
