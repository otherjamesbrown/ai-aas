output "instance_id" {
  description = "Linode instance ID"
  value       = linode_instance.workspace.id
}

output "instance_label" {
  description = "Instance label"
  value       = linode_instance.workspace.label
}

output "private_ip" {
  description = "Private IP address (VLAN) if configured"
  value       = try(linode_instance.workspace.private_ip_address, "")
}

output "public_ip" {
  description = "Public IP address"
  value       = linode_instance.workspace.ip_address
}

output "ipv6" {
  description = "IPv6 address"
  value       = linode_instance.workspace.ipv6
}

output "ssh_command" {
  description = "SSH command to connect to the workspace"
  value       = "ssh root@${linode_instance.workspace.ip_address}"
}

output "workspace_name" {
  description = "Workspace identifier"
  value       = var.workspace_name
}

output "owner" {
  description = "Workspace owner"
  value       = var.owner
}

output "region" {
  description = "Linode region"
  value       = var.region
}

output "ttl_timestamp" {
  description = "TTL expiration timestamp (RFC3339)"
  value       = timeadd(timestamp(), "${var.ttl_hours}h")
}

output "stackscript_id" {
  description = "StackScript ID used for bootstrap"
  value       = var.stackscript_id != null ? var.stackscript_id : linode_stackscript.bootstrap[0].id
}

output "tags" {
  description = "Instance tags"
  value       = linode_instance.workspace.tags
}

output "status" {
  description = "Instance status"
  value       = linode_instance.workspace.status
}

