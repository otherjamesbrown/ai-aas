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

# Dev workspace variables (optional)
variable "enable_dev_workspace" {
  description = "Enable dev workspace provisioning (default: false)"
  type        = bool
  default     = false
}

variable "workspace_name" {
  description = "Workspace name identifier (e.g., dev-jab)"
  type        = string
  default     = ""
}

variable "workspace_owner" {
  description = "Workspace owner (GitHub username)"
  type        = string
  default     = ""
}

variable "workspace_ttl_hours" {
  description = "Workspace TTL in hours (default: 24)"
  type        = number
  default     = 24
}

variable "workspace_instance_type" {
  description = "Linode instance type for workspace (default: g6-nanode-1)"
  type        = string
  default     = "g6-nanode-1"
}

variable "workspace_vlan_id" {
  description = "Private VLAN ID for workspace (optional)"
  type        = string
  default     = ""
}

variable "workspace_ssh_keys" {
  description = "List of Linode SSH key IDs for workspace"
  type        = list(string)
  default     = []
}

variable "workspace_authorized_keys" {
  description = "List of SSH public keys to authorize on workspace"
  type        = list(string)
  default     = []
}
