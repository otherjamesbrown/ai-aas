locals {
  values_content = templatefile("${path.module}/templates/values.yaml.tmpl", {
    environment   = var.environment
    retention     = var.retention_days
    slack_channel = var.alert_slack_channel
  })
  output_path = "${var.output_dir}/observability/${var.environment}-values.yaml"
}

resource "null_resource" "prepare" {
  provisioner "local-exec" {
    command = "mkdir -p ${var.output_dir}/observability"
  }
}

resource "local_file" "values" {
  content    = local.values_content
  filename   = local.output_path
  depends_on = [null_resource.prepare]
}

output "values_file" {
  value       = local.output_path
  description = "Helm values file for kube-prometheus-stack"
}
