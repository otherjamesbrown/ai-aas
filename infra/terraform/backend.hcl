bucket                      = "ai-aas"
key                         = "terraform/environments/production/terraform.tfstate"
region                      = "fr-par-1"
skip_credentials_validation = true
skip_region_validation      = true
skip_metadata_api_check     = true
skip_requesting_account_id  = true
use_path_style              = true
skip_s3_checksum            = true
endpoints = {
  s3 = "https://fr-par-1.linodeobjects.com"
}
