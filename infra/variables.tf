variable "project_id" {
  description = "Existing GCP project with billing enabled."
  type        = string
}

variable "region" {
  description = "Cloud Run deployment region."
  type        = string
  default     = "us-east1"
}

variable "service_name" {
  description = "Cloud Run service and dedicated runtime service account name."
  type        = string
  default     = "manifests-io"
  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{4,28}[a-z0-9]$", var.service_name))
    error_message = "Use a lowercase service account name of 6 to 30 characters."
  }
}

variable "site_url" {
  description = "Canonical HTTPS origin. Set to the Cloud Run URL for a preview deployment."
  type        = string
  default     = "https://www.manifests.io"
  validation {
    condition     = can(regex("^https://[^/@?#:]+(:[0-9]+)?$", var.site_url))
    error_message = "Use an HTTPS origin without a path, credentials, query, or fragment."
  }
}

variable "image" {
  description = "Existing linux/amd64 container image pinned to its immutable SHA256 digest."
  type        = string
  validation {
    condition     = can(regex("^[^[:space:]]+@sha256:[a-f0-9]{64}$", var.image))
    error_message = "Pin the image with @sha256:<64 lowercase hex characters>."
  }
}

variable "max_instances" {
  description = "Upper bound on Cloud Run instances."
  type        = number
  default     = 5
  validation {
    condition     = var.max_instances >= 1 && var.max_instances <= 20 && floor(var.max_instances) == var.max_instances
    error_message = "Choose an integer from 1 to 20."
  }
}

variable "otlp_endpoint" {
  description = "OTLP HTTP base endpoint without credentials or a signal-specific path."
  type        = string
  default     = "https://otlp-gateway-prod-us-east-3.grafana.net/otlp"
  validation {
    condition     = can(regex("^https://[^@/?#]+(/[^?#]*)?$", var.otlp_endpoint))
    error_message = "Use an HTTPS endpoint without embedded credentials, queries, or fragments."
  }
}

variable "otlp_headers_secret" {
  description = "Existing Secret Manager secret ID in this project containing OTLP headers."
  type        = string
}

variable "otlp_headers_secret_version" {
  description = "Pinned numeric version of the existing OTLP header secret."
  type        = string
  validation {
    condition     = can(regex("^[1-9][0-9]*$", var.otlp_headers_secret_version))
    error_message = "Pin a numeric secret version for reproducible revisions."
  }
}
