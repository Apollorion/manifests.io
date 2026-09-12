mock_provider "google" {}

variables {
  project_id                  = "test-project"
  image                       = "us-east1-docker.pkg.dev/test/apps/manifests@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  otlp_headers_secret         = "telemetry"
  otlp_headers_secret_version = "1"
  manage_project_apis          = false
}

run "bootstrap_preserves_cloud_run" {
  command = plan
  variables {
    static_bucket_name = "test-manifests-static"
  }
  assert {
    condition     = length(google_cloud_run_v2_service.app) == 1 && google_cloud_run_v2_service.app[0].deletion_protection
    error_message = "Creating static storage must preserve the protected Cloud Run origin."
  }
  assert {
    condition     = google_storage_bucket.static[0].uniform_bucket_level_access && !google_storage_bucket.static[0].force_destroy && google_storage_bucket_iam_member.static_public[0].role == "roles/storage.legacyObjectReader"
    error_message = "Static storage must allow known-object reads without public listing or forced bucket deletion."
  }
  assert {
    condition     = length(google_project_service.required) == 0
    error_message = "Shared API ownership must remain outside the application module."
  }
}

run "disable_protection_before_retirement" {
  command = plan
  variables {
    cloud_run_deletion_protection = false
  }
  assert {
    condition     = length(google_cloud_run_v2_service.app) == 1 && !google_cloud_run_v2_service.app[0].deletion_protection
    error_message = "Deletion protection must be removable without removing the origin."
  }
}

run "static_only_removes_runtime" {
  command = plan
  variables {
    static_bucket_name           = "test-manifests-static"
    cloud_run_enabled            = false
    cloud_run_deletion_protection = false
    image                        = ""
  }
  assert {
    condition     = length(google_cloud_run_v2_service.app) == 0 && length(google_service_account.runtime) == 0 && length(google_secret_manager_secret_iam_member.telemetry) == 0 && length(google_cloud_run_v2_service_iam_member.public) == 0
    error_message = "Static-only deployment must remove all runtime resources and permissions."
  }
  assert {
    condition     = length(google_storage_bucket.static) == 1 && output.url == null && output.runtime_service_account == null
    error_message = "Static storage must remain available after retiring the origin."
  }
}
