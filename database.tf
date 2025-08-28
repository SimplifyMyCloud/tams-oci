resource "oci_core_instance" "tams_database" {
  availability_domain = local.availability_domain
  compartment_id      = var.compartment_ocid
  shape               = var.instance_shape

  shape_config {
    ocpus         = 1
    memory_in_gbs = 8
  }

  display_name = "${var.project_name}-postgresql-db"

  create_vnic_details {
    subnet_id        = oci_core_subnet.tams_db_subnet.id
    display_name     = "${var.project_name}-db-vnic"
    assign_public_ip = false
    hostname_label   = "tamsdb"
  }

  source_details {
    source_type = "image"
    source_id   = data.oci_core_images.ubuntu.images[0].id
  }

  metadata = {
    ssh_authorized_keys = file(var.ssh_public_key_path)
    user_data = base64encode(templatefile("${path.module}/scripts/init_postgresql.sh", {
      DB_PASSWORD   = var.db_admin_password
      DB_VERSION    = var.db_version
      BACKUP_BUCKET = oci_objectstorage_bucket.tams_archive_bucket.name
      NAMESPACE     = data.oci_objectstorage_namespace.ns.namespace
    }))
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
    Role        = "Database"
    Type        = "PostgreSQL"
  }
}

resource "oci_core_volume" "tams_db_volume" {
  availability_domain = local.availability_domain
  compartment_id      = var.compartment_ocid
  display_name        = "${var.project_name}-db-volume"
  size_in_gbs         = 500
  vpus_per_gb         = 20

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
    Purpose     = "Database storage"
  }
}

resource "oci_core_volume_attachment" "tams_db_volume_attachment" {
  attachment_type = "paravirtualized"
  instance_id     = oci_core_instance.tams_database.id
  volume_id       = oci_core_volume.tams_db_volume.id
  display_name    = "${var.project_name}-db-volume-attachment"
  device          = "/dev/oracleoci/oraclevdb"
}

resource "oci_core_volume_backup_policy" "tams_db_backup_policy" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.project_name}-db-backup-policy"

  schedules {
    backup_type       = "INCREMENTAL"
    period            = "ONE_DAY"
    retention_seconds = 604800
    time_zone         = "UTC"
    hour_of_day       = 2
  }

  schedules {
    backup_type       = "FULL"
    period            = "ONE_WEEK"
    retention_seconds = 2592000
    time_zone         = "UTC"
    hour_of_day       = 3
    day_of_week       = "SUNDAY"
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

resource "oci_core_volume_backup_policy_assignment" "tams_db_backup_assignment" {
  asset_id  = oci_core_volume.tams_db_volume.id
  policy_id = oci_core_volume_backup_policy.tams_db_backup_policy.id
}