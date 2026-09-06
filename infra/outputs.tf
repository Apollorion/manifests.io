output "url" {
  description = "Public Cloud Run URL for smoke verification."
  value       = google_cloud_run_v2_service.app.uri
}

output "runtime_service_account" {
  description = "Dedicated runtime identity with access only to the OTLP header secret."
  value       = google_service_account.runtime.email
}
