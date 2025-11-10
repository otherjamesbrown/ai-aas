locals {
  output_path = "${var.output_dir}/secrets/${var.environment}-bootstrap.yaml"
  manifest    = templatefile("${path.module}/templates/sealed-secret.yaml.tmpl", {
    environment = var.environment
    secrets     = var.bootstrap_secrets
  })
}

resource "null_resource" "prepare" {
  provisioner "local-exec" {
    command = "mkdir -p ${var.output_dir}/secrets"
  }
}

resource "local_sensitive_file" "sealed_secret" {
  content              = local.manifest
  filename             = local.output_path
  file_permission      = "0600"
  directory_permission = "0700"
  depends_on           = [null_resource.prepare]
}

output "sealed_secret_file" {
  value       = local.output_path
  description = "Rendered SealedSecret manifest"
  sensitive   = true
}
