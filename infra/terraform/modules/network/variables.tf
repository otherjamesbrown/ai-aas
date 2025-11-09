variable "environment" {
  description = "Environment name"
  type        = string
}

variable "output_dir" {
  description = "Directory where rendered manifests will be written"
  type        = string
}

variable "ingress_whitelist" {
  description = "CIDR blocks allowed to reach ingress"
  type        = list(string)
  default     = []
}

variable "allowed_egress_cidrs" {
  description = "CIDRs allowed for egress"
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "namespace" {
  description = "Namespace associated with the environment"
  type        = string
  default     = "default"
}
