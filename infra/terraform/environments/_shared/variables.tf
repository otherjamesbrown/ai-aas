variable "environment" {
  description = "Logical environment name (development, staging, production, system)"
  type        = string
}

variable "project" {
  description = "Short project identifier used in resource tags"
  type        = string
  default     = "ai-aas"
}

variable "linode_token" {
  description = "Linode API token (optional when using environment variables)"
  type        = string
  default     = ""
  sensitive   = true
}

variable "kubeconfig_path" {
  description = "Path to kubeconfig for the environment"
  type        = string
  default     = ""
}

variable "kubeconfig_context" {
  description = "Kubeconfig context to use for Kubernetes/Helm providers"
  type        = string
  default     = ""
}

variable "tags" {
  description = "Additional tags to apply to created resources"
  type        = map(string)
  default     = {}
}
variable "default_region" {
  description = "Default Linode region to deploy infrastructure"
  type        = string
  default     = "fr-par"
}

variable "region_overrides" {
  description = "Optional map of environment name to region overrides"
  type        = map(string)
  default     = {}
}
