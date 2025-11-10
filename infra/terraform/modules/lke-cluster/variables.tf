variable "cluster_label" {
  description = "Cluster label"
  type        = string
}

variable "region" {
  description = "Linode region"
  type        = string
}

variable "k8s_version" {
  description = "Kubernetes version"
  type        = string
}

variable "tags" {
  description = "Tags assigned to the cluster"
  type        = list(string)
  default     = []
}

variable "node_pools" {
  description = "List of node pool definitions"
  type = list(object({
    type  = string
    count = number
    autoscaler = object({
      min = number
      max = number
    })
  }))
}
