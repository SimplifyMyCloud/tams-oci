resource "oci_core_instance_configuration" "tams_api_config" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.project_name}-api-config"

  instance_details {
    instance_type = "compute"

    launch_details {
      compartment_id = var.compartment_ocid
      shape          = var.instance_shape

      shape_config {
        ocpus         = var.instance_ocpus
        memory_in_gbs = var.instance_memory_in_gbs
      }

      create_vnic_details {
        subnet_id        = oci_core_subnet.tams_private_subnet.id
        hostname_label   = "tamsapi"
        assign_public_ip = false
        nsg_ids          = [oci_core_network_security_group.tams_api_nsg.id]
      }

      source_details {
        source_type = "image"
        image_id    = data.oci_core_images.ubuntu.images[0].id
      }

      metadata = {
        ssh_authorized_keys = file(var.ssh_public_key_path)
        user_data = base64encode(templatefile("${path.module}/scripts/init_tams_api.sh", {
          DB_HOST        = oci_core_instance.tams_database.private_ip
          DB_PASSWORD    = var.db_admin_password
          MEDIA_BUCKET   = oci_objectstorage_bucket.tams_media_bucket.name
          ARCHIVE_BUCKET = oci_objectstorage_bucket.tams_archive_bucket.name
          TEMP_BUCKET    = oci_objectstorage_bucket.tams_temp_bucket.name
          NAMESPACE      = data.oci_objectstorage_namespace.ns.namespace
          REGION         = var.region
        }))
      }
    }
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
    Role        = "API"
  }
}

resource "oci_core_instance_pool" "tams_api_pool" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.project_name}-api-pool"
  size           = var.api_instance_count

  instance_configuration_id = oci_core_instance_configuration.tams_api_config.id

  placement_configurations {
    availability_domain = local.availability_domain
    primary_subnet_id   = oci_core_subnet.tams_private_subnet.id
  }

  load_balancers {
    backend_set_name = oci_load_balancer_backend_set.tams_api_backend_set.name
    load_balancer_id = oci_load_balancer.tams_load_balancer.id
    port             = 8080
    vnic_selection   = "PrimaryVnic"
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

resource "oci_autoscaling_auto_scaling_configuration" "tams_api_autoscaling" {
  compartment_id       = var.compartment_ocid
  display_name         = "${var.project_name}-api-autoscaling"
  is_enabled           = true
  cool_down_in_seconds = 300

  auto_scaling_resources {
    id   = oci_core_instance_pool.tams_api_pool.id
    type = "instancePool"
  }

  policies {
    display_name = "${var.project_name}-api-scaling-policy"
    policy_type  = "threshold"
    capacity {
      initial = 2
      max     = 10
      min     = 2
    }

    rules {
      display_name = "scale-out-rule"
      action {
        type  = "CHANGE_COUNT_BY"
        value = 2
      }
      metric {
        metric_type = "CPU_UTILIZATION"
        threshold {
          operator = "GT"
          value    = 80
        }
      }
    }

    rules {
      display_name = "scale-in-rule"
      action {
        type  = "CHANGE_COUNT_BY"
        value = -1
      }
      metric {
        metric_type = "CPU_UTILIZATION"
        threshold {
          operator = "LT"
          value    = 20
        }
      }
    }
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

resource "oci_load_balancer" "tams_load_balancer" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.project_name}-lb"
  shape          = "flexible"

  shape_details {
    minimum_bandwidth_in_mbps = 10
    maximum_bandwidth_in_mbps = 100
  }

  subnet_ids = [oci_core_subnet.tams_public_subnet.id]

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

resource "oci_load_balancer_backend_set" "tams_api_backend_set" {
  load_balancer_id = oci_load_balancer.tams_load_balancer.id
  name             = "${var.project_name}-api-backend-set"
  policy           = "ROUND_ROBIN"

  health_checker {
    protocol          = "HTTP"
    port              = 8080
    url_path          = "/health"
    return_code       = 200
    interval_ms       = 10000
    timeout_in_millis = 3000
    retries           = 3
  }

  session_persistence_configuration {
    cookie_name      = "tams-session"
    disable_fallback = false
  }
}


resource "oci_load_balancer_certificate" "tams_certificate" {
  load_balancer_id   = oci_load_balancer.tams_load_balancer.id
  certificate_name   = "${var.project_name}-cert"
  public_certificate = file("${path.module}/certs/tams.crt")
  private_key        = file("${path.module}/certs/tams.key")

  lifecycle {
    create_before_destroy = true
  }
}