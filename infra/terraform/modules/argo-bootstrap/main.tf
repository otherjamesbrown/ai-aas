locals {
  appset_path = "${var.output_dir}/argo/${var.environment}-applicationset.yaml"
}

resource "null_resource" "prepare" {
  provisioner "local-exec" {
    command = "mkdir -p ${var.output_dir}/argo"
  }
}

resource "local_file" "applicationset" {
  filename   = local.appset_path
  content    = templatefile("${path.module}/templates/applicationset.yaml.tmpl", {
    environment = var.environment
    repo_url    = var.repo_url
    revision    = var.revision
    path        = var.path
  })
  depends_on = [null_resource.prepare]
}

output "applicationset_file" {
  value       = local.appset_path
  description = "Rendered ArgoCD ApplicationSet manifest"
}
