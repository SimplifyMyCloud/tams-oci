resource "oci_objectstorage_namespace" "tams_namespace" {
  compartment_id = var.compartment_ocid
}

resource "oci_objectstorage_bucket" "tams_media_bucket" {
  compartment_id = var.compartment_ocid
  namespace      = data.oci_objectstorage_namespace.ns.namespace
  name           = "${var.project_name}-media-${var.environment}"
  access_type    = "NoPublicAccess"
  storage_tier   = "Standard"
  versioning     = "Enabled"

  object_events_enabled = true

  retention_rules {
    display_name = "tams-media-retention"
    duration {
      time_amount = 7
      time_unit   = "YEARS"
    }
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
    Purpose     = "TAMS media chunks storage"
  }
}

resource "oci_objectstorage_bucket" "tams_archive_bucket" {
  compartment_id = var.compartment_ocid
  namespace      = data.oci_objectstorage_namespace.ns.namespace
  name           = "${var.project_name}-archive-${var.environment}"
  access_type    = "NoPublicAccess"
  storage_tier   = "Archive"

  object_events_enabled = true
  auto_tiering          = "InfrequentAccess"

  retention_rules {
    display_name = "tams-archive-retention"
    duration {
      time_amount = 10
      time_unit   = "YEARS"
    }
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
    Purpose     = "TAMS long-term archive storage"
  }
}

resource "oci_objectstorage_bucket" "tams_temp_bucket" {
  compartment_id = var.compartment_ocid
  namespace      = data.oci_objectstorage_namespace.ns.namespace
  name           = "${var.project_name}-temp-${var.environment}"
  access_type    = "NoPublicAccess"
  storage_tier   = "Standard"

  lifecycle_rules {
    name        = "delete-temp-objects"
    enabled     = true
    time_amount = 7
    time_unit   = "DAYS"
    action      = "DELETE"
    is_locked   = false

    object_name_filter {
      inclusion_prefixes = ["temp/"]
    }
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
    Purpose     = "TAMS temporary processing storage"
  }
}

data "oci_objectstorage_namespace" "ns" {
  compartment_id = var.compartment_ocid
}

resource "oci_identity_policy" "tams_storage_policy" {
  compartment_id = var.compartment_ocid
  name           = "${var.project_name}-storage-policy"
  description    = "Policy for TAMS storage access"

  statements = [
    "Allow group ${var.project_name}-api-group to manage objects in compartment id ${var.compartment_ocid} where target.bucket.name='${oci_objectstorage_bucket.tams_media_bucket.name}'",
    "Allow group ${var.project_name}-api-group to manage objects in compartment id ${var.compartment_ocid} where target.bucket.name='${oci_objectstorage_bucket.tams_archive_bucket.name}'",
    "Allow group ${var.project_name}-api-group to manage objects in compartment id ${var.compartment_ocid} where target.bucket.name='${oci_objectstorage_bucket.tams_temp_bucket.name}'",
  ]

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}