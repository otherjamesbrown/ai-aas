variable "environment" {
  description = "Environment name"
  type        = string
}

variable "output_dir" {
  description = "Directory to place rendered Helm values"
  type        = string
}

variable "retention_days" {
  description = "Retention window for metrics/logs"
  type        = number
  default     = 30
}

variable "alert_slack_channel" {
  description = "Slack channel for alerts"
  type        = string
  default     = "#platform-infra"
}
