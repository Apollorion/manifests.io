output "url" {
  description = "Public Cloud Run URL for smoke verification."
  value       = try(google_cloud_run_v2_service.app[0].uri, null)
}

output "runtime_service_account" {
  description = "Dedicated runtime identity with access only to the OTLP header secret."
  value       = try(google_service_account.runtime[0].email, null)
}

output "static_bucket_name" {
  description = "Bucket for immutable static objects and the atomic current.json release pointer."
  value       = try(google_storage_bucket.static[0].name, null)
  depends_on  = [google_storage_bucket_iam_member.static_public]
}
