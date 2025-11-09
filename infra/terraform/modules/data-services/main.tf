locals {
  doc_path = "${var.output_dir}/data-services/${var.environment}-endpoints.md"
}

resource "null_resource" "prepare" {
  provisioner "local-exec" {
    command = "mkdir -p ${var.output_dir}/data-services"
  }
}

resource "local_file" "documentation" {
  filename   = local.doc_path
  content    = templatefile("${path.module}/templates/endpoints.md.tmpl", {
    environment = var.environment
    endpoints   = var.endpoints
  })
  depends_on = [null_resource.prepare]
}

output "endpoints_doc" {
  value       = local.doc_path
  description = "Rendered shared service endpoint documentation"
}

output "endpoints" {
  value       = var.endpoints
  description = "Endpoints map"
}
