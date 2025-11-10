variable "environment" {
  description = "Environment name"
  type        = string
}

variable "output_dir" {
  description = "Directory to place ArgoCD manifests"
  type        = string
}

variable "repo_url" {
  description = "Repository URL tracked by ArgoCD"
  type        = string
}

variable "revision" {
  description = "Git revision"
  type        = string
  default     = "main"
}

variable "path" {
  description = "Path within repo for manifests"
  type        = string
  default     = "infra/helm"
}
