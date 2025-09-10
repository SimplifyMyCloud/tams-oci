output "load_balancer_ip" {
  description = "Public IP of the load balancer"
  value       = oci_load_balancer.tams_load_balancer.ip_address_details[0].ip_address
}

output "web_ui_url" {
  description = "URL to access the TAMS Web UI"
  value       = "https://${oci_load_balancer.tams_load_balancer.ip_address_details[0].ip_address}"
}

output "web_ui_private_ip" {
  description = "Private IP of the Web UI instance"
  value       = oci_core_instance.tams_webui.private_ip
}

output "database_private_ip" {
  description = "Private IP of the database instance"
  value       = oci_core_instance.tams_database.private_ip
  sensitive   = true
}

output "media_bucket_name" {
  description = "Name of the media storage bucket"
  value       = oci_objectstorage_bucket.tams_media_bucket.name
}

output "archive_bucket_name" {
  description = "Name of the archive storage bucket"
  value       = oci_objectstorage_bucket.tams_archive_bucket.name
}

output "temp_bucket_name" {
  description = "Name of the temporary storage bucket"
  value       = oci_objectstorage_bucket.tams_temp_bucket.name
}

output "vcn_id" {
  description = "VCN OCID"
  value       = oci_core_vcn.tams_vcn.id
}

output "api_subnet_id" {
  description = "API subnet OCID"
  value       = oci_core_subnet.tams_private_subnet.id
}

output "db_subnet_id" {
  description = "Database subnet OCID"
  value       = oci_core_subnet.tams_db_subnet.id
}

output "instance_pool_id" {
  description = "Instance pool OCID"
  value       = oci_core_instance_pool.tams_api_pool.id
}

output "namespace" {
  description = "Object storage namespace"
  value       = data.oci_objectstorage_namespace.ns.namespace
}