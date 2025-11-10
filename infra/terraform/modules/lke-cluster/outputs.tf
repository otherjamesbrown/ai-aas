output "cluster_id" {
  value       = linode_lke_cluster.this.id
  description = "Cluster ID"
}

output "api_endpoints" {
  value       = linode_lke_cluster.this.api_endpoints
  description = "Kubernetes API endpoints"
}

output "status" {
  value       = linode_lke_cluster.this.status
  description = "Cluster status"
}

