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

output "kubeconfig_raw" {
  value       = data.linode_lke_cluster_kubeconfig.this.kubeconfig
  description = "Base64 encoded kubeconfig"
  sensitive   = true
}

output "dashboard_url" {
  value       = data.linode_lke_cluster_dashboard.this.url
  description = "Dashboard URL"
}
