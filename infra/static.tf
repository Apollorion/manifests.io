resource "google_storage_bucket" "static" {
  count                       = var.static_bucket_name != "" ? 1 : 0
  project                     = var.project_id
  name                        = var.static_bucket_name
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "inherited"
  force_destroy               = false

  soft_delete_policy {
    retention_duration_seconds = 604800
  }

  depends_on = [google_project_service.required]
}

resource "google_storage_bucket_iam_member" "static_public" {
  count  = var.static_bucket_name != "" ? 1 : 0
  bucket = google_storage_bucket.static[0].name
  role   = "roles/storage.legacyObjectReader"
  member = "allUsers"
}
