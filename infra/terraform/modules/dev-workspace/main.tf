# Dev Workspace Module
# Provisions an ephemeral Linode instance for remote development workspace.
# Includes StackScript bootstrap, private VLAN attachment, and security tags.

locals {
  # Standard tags for workspace identification and cleanup
  base_tags = concat([
    "dev-workspace",
    "workspace:${var.workspace_name}",
    "owner:${var.owner}",
    "ttl-hours:${var.ttl_hours}",
    "managed-by:terraform",
    "provisioner:dev-workspace-module"
  ], var.tags)

  # StackScript data with workspace metadata
  script_data = merge({
    workspace_name = var.workspace_name
    owner          = var.owner
    ttl_hours      = tostring(var.ttl_hours)
  }, var.stackscript_data)

  # Instance label
  instance_label = "dev-${var.workspace_name}"

  # TTL timestamp for cleanup automation
  ttl_timestamp = timeadd(timestamp(), "${var.ttl_hours}h")
}

# StackScript for bootstrap (creates StackScript if file is provided)
resource "linode_stackscript" "bootstrap" {
  count = var.stackscript_id == null ? 1 : 0

  label       = "dev-workspace-bootstrap-${var.workspace_name}"
  description = "Bootstrap script for dev workspace ${var.workspace_name}"
  script      = file("${path.module}/files/bootstrap.sh")
  images      = ["linode/ubuntu22.04", "linode/ubuntu20.04"]
  is_public   = false
  rev_note    = "Initial bootstrap script for workspace provisioning"

  lifecycle {
    create_before_destroy = true
  }
}

# Linode Instance
resource "linode_instance" "workspace" {
  label           = local.instance_label
  region          = var.region
  type            = var.instance_type
  image           = var.image
  root_pass       = var.root_pass != null ? var.root_pass : random_password.root_pass.result
  authorized_keys = var.authorized_keys
  swap_size       = var.swap_size
  backups_enabled = var.backup_enabled
  backup_schedule {
    day    = var.backup_schedule.day
    window = var.backup_schedule.window
  }
  watchdog_enabled = var.watchdog_enabled

  stackscript_id  = var.stackscript_id != null ? var.stackscript_id : linode_stackscript.bootstrap[0].id
  stackscript_data = local.script_data

  # Security: Firewall should be managed separately
  # Use private VLAN for internal networking
  interfaces {
    purpose = "public"
  }

  dynamic "interfaces" {
    for_each = var.vlan_id != "" ? [1] : []
    content {
      purpose = "vlan"
      label   = var.vlan_id
    }
  }

  tags = local.base_tags

  lifecycle {
    # Prevent accidental deletion without explicit destroy
    prevent_destroy = false
    # Allow updates to instance config without recreation
    ignore_changes = [authorized_keys]
  }
}

# Random password for root if not provided
resource "random_password" "root_pass" {
  length  = 32
  special = true
  keepers = {
    workspace = var.workspace_name
  }
}

# VLAN resource (if creating new VLAN)
resource "linode_vlan" "workspace_vlan" {
  count  = var.vlan_id == "" ? 0 : 1
  label  = var.vlan_id
  region = var.region
}

