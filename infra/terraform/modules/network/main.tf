locals {
  manifest_content = templatefile(
    "${path.module}/templates/network-policy.yaml.tmpl",
    {
      namespace        = var.namespace
      environment      = var.environment
      allowed_egress   = var.allowed_egress_cidrs
      ingress_whitelist = var.ingress_whitelist
    }
  )

  firewall_content = templatefile(
    "${path.module}/templates/firewall.json.tmpl",
    {
      environment = var.environment
      ingress     = var.ingress_whitelist
      egress      = var.allowed_egress_cidrs
    }
  )

  manifest_path = "${var.output_dir}/network/${var.environment}-network-policy.yaml"
  firewall_path = "${var.output_dir}/network/${var.environment}-firewall.json"
}

resource "null_resource" "prepare" {
  provisioner "local-exec" {
    command = "mkdir -p ${var.output_dir}/network"
  }
}

resource "local_file" "network_policy" {
  content  = local.manifest_content
  filename = local.manifest_path
  depends_on = [null_resource.prepare]
}

resource "local_file" "firewall" {
  content  = local.firewall_content
  filename = local.firewall_path
  depends_on = [null_resource.prepare]
}

output "network_policy_file" {
  value       = local.manifest_path
  description = "Rendered NetworkPolicy manifest"
}

output "firewall_spec_file" {
  value       = local.firewall_path
  description = "Rendered firewall specification"
}
