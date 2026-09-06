terraform {
  required_version = ">= 1.10, < 2.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 7.0, < 8.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

resource "google_project_service" "required" {
  for_each           = toset(["run.googleapis.com", "secretmanager.googleapis.com"])
  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

resource "google_service_account" "runtime" {
  project      = var.project_id
  account_id   = var.service_name
  display_name = "Manifests.io Cloud Run runtime"
}

resource "google_secret_manager_secret_iam_member" "telemetry" {
  project   = var.project_id
  secret_id = var.otlp_headers_secret
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.runtime.email}"
}

resource "google_cloud_run_v2_service" "app" {
  project             = var.project_id
  name                = var.service_name
  location            = var.region
  deletion_protection = true
  ingress             = "INGRESS_TRAFFIC_ALL"

  template {
    service_account                  = google_service_account.runtime.email
    timeout                          = "30s"
    max_instance_request_concurrency = 40

    scaling {
      min_instance_count = 0
      max_instance_count = var.max_instances
    }

    containers {
      image = var.image
      ports {
        container_port = 8080
      }
      resources {
        limits = {
          cpu    = "1"
          memory = "1Gi"
        }
        cpu_idle          = false
        startup_cpu_boost = true
      }
      startup_probe {
        period_seconds    = 2
        timeout_seconds   = 1
        failure_threshold = 60
        http_get {
          path = "/readyz"
          port = 8080
        }
      }
      liveness_probe {
        period_seconds    = 30
        timeout_seconds   = 1
        failure_threshold = 3
        http_get {
          path = "/healthz"
          port = 8080
        }
      }
      env {
        name  = "OTEL_EXPORTER_OTLP_ENDPOINT"
        value = var.otlp_endpoint
      }
      env {
        name  = "OTEL_EXPORTER_OTLP_PROTOCOL"
        value = "http/protobuf"
      }
      env {
        name  = "OTEL_BSP_SCHEDULE_DELAY"
        value = "1000"
      }
      env {
        name  = "OTEL_BLRP_SCHEDULE_DELAY"
        value = "1000"
      }
      env {
        name = "OTEL_EXPORTER_OTLP_HEADERS"
        value_source {
          secret_key_ref {
            secret  = var.otlp_headers_secret
            version = var.otlp_headers_secret_version
          }
        }
      }
    }
  }

  depends_on = [google_project_service.required, google_secret_manager_secret_iam_member.telemetry]
}

resource "google_cloud_run_v2_service_iam_member" "public" {
  project  = var.project_id
  location = google_cloud_run_v2_service.app.location
  name     = google_cloud_run_v2_service.app.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
