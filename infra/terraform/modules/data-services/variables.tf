variable "environment" {
  description = "Environment name"
  type        = string
}

variable "output_dir" {
  description = "Directory to place generated documentation"
  type        = string
}

variable "endpoints" {
  description = "Map describing shared services (postgres, redis, rabbitmq, etc.)"
  type = map(object({
    host = string
    port = number
    protocol = string
  }))
}
