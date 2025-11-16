variable "workspace_name" {
  description = "Unique identifier for the workspace (e.g., 'dev-jab')"
  type        = string
}

variable "region" {
  description = "Linode region for the workspace instance"
  type        = string
}

variable "instance_type" {
  description = "Linode instance type (default: g6-nanode-1 for cost efficiency)"
  type        = string
  default     = "g6-nanode-1"
}

variable "image" {
  description = "Linode image to use (default: Ubuntu 22.04 LTS)"
  type        = string
  default     = "linode/ubuntu22.04"
}

variable "ttl_hours" {
  description = "Time-to-live for workspace in hours (default: 24)"
  type        = number
  default     = 24
}

variable "owner" {
  description = "Workspace owner (GitHub username or identifier)"
  type        = string
}

variable "vlan_id" {
  description = "Private VLAN ID for workspace networking (optional)"
  type        = string
  default     = ""
}

variable "ssh_keys" {
  description = "List of SSH key IDs to install on the instance"
  type        = list(string)
  default     = []
}

variable "stackscript_id" {
  description = "Linode StackScript ID for bootstrap (if using existing StackScript)"
  type        = number
  default     = null
}

variable "stackscript_data" {
  description = "Data to pass to StackScript"
  type        = map(string)
  default     = {}
}

variable "tags" {
  description = "Additional tags for the instance"
  type        = list(string)
  default     = []
}

variable "root_pass" {
  description = "Root password for the instance (auto-generated if not provided)"
  type        = string
  default     = null
  sensitive   = true
}

variable "authorized_keys" {
  description = "List of SSH public keys to authorize for root user"
  type        = list(string)
  default     = []
}

variable "swap_size" {
  description = "Swap disk size in MB"
  type        = number
  default     = 512
}

variable "backup_enabled" {
  description = "Enable Linode backups (default: false for ephemeral workspaces)"
  type        = bool
  default     = false
}

variable "backup_schedule" {
  description = "Backup schedule (daily, weekly, biweekly)"
  type = object({
    day    = string
    window = string
  })
  default = {
    day    = "Sunday"
    window = "W0"
  }
}

variable "watchdog_enabled" {
  description = "Enable Linode Watchdog service"
  type        = bool
  default     = true
}

