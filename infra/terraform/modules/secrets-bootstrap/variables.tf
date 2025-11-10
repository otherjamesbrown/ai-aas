variable "environment" {
  description = "Environment name"
  type        = string
}

variable "output_dir" {
  description = "Directory where rendered secret manifests will be written"
  type        = string
}

variable "bootstrap_secrets" {
  description = "Map of secret names to key/value pairs"
  type = map(object({
    data         = map(string)
    annotations  = optional(map(string), {})
    rotation_days = optional(number, 30)
  }))
}
