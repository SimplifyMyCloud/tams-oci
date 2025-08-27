variable "tenancy_ocid" {
  description = "OCI Tenancy OCID"
  type        = string
}

variable "user_ocid" {
  description = "OCI User OCID"
  type        = string
}

variable "fingerprint" {
  description = "OCI API Key fingerprint"
  type        = string
}

variable "private_key_path" {
  description = "Path to OCI API private key"
  type        = string
}

variable "region" {
  description = "OCI region"
  type        = string
  default     = "us-phoenix-1"
}

variable "compartment_ocid" {
  description = "OCI Compartment OCID for TAMS resources"
  type        = string
}

variable "project_name" {
  description = "Project name for tagging"
  type        = string
  default     = "tams"
}

variable "environment" {
  description = "Environment name"
  type        = string
  default     = "production"
}

variable "availability_domain_number" {
  description = "Availability domain number (1, 2, or 3)"
  type        = number
  default     = 1
}

variable "ssh_public_key" {
  description = "SSH public key for instance access"
  type        = string
}

variable "db_admin_password" {
  description = "Database admin password"
  type        = string
  sensitive   = true
}

variable "db_version" {
  description = "PostgreSQL database version"
  type        = string
  default     = "14"
}

variable "instance_shape" {
  description = "Compute instance shape"
  type        = string
  default     = "VM.Standard.E4.Flex"
}

variable "instance_ocpus" {
  description = "Number of OCPUs for flexible shapes"
  type        = number
  default     = 2
}

variable "instance_memory_in_gbs" {
  description = "Memory in GBs for flexible shapes"
  type        = number
  default     = 16
}

variable "api_instance_count" {
  description = "Number of API instances"
  type        = number
  default     = 2
}

variable "web_domain" {
  description = "Domain name for the web UI (optional)"
  type        = string
  default     = ""
}