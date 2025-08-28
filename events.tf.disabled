# OCI Events Rule for Object Storage uploads
resource "oci_events_rule" "video_upload_rule" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.project_name}-video-upload-rule"
  description    = "Triggers when videos are uploaded to media bucket"
  is_enabled     = true

  condition = jsonencode({
    "eventType" : ["com.oraclecloud.objectstorage.object.create"],
    "data" : {
      "additionalDetails" : {
        "bucketName" : [oci_objectstorage_bucket.tams_media_bucket.name]
      }
    }
  })

  actions {
    actions {
      action_type = "FAAS"
      is_enabled  = true

      function_id = oci_functions_function.video_processor.id
    }
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

# Notification topic for processing updates
resource "oci_ons_notification_topic" "tams_processing_topic" {
  compartment_id = var.compartment_ocid
  name           = "${var.project_name}-processing-notifications"
  description    = "Topic for TAMS video processing notifications"

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

# Email subscription for processing notifications (optional)
resource "oci_ons_subscription" "admin_email_subscription" {
  count          = var.admin_email != "" ? 1 : 0
  compartment_id = var.compartment_ocid
  topic_id       = oci_ons_notification_topic.tams_processing_topic.id
  endpoint       = var.admin_email
  protocol       = "EMAIL"

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

# Additional Events Rule for processing completion (from temp bucket)
resource "oci_events_rule" "chunk_completion_rule" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.project_name}-chunk-completion-rule"
  description    = "Triggers when chunking is complete in temp bucket"
  is_enabled     = true

  condition = jsonencode({
    "eventType" : ["com.oraclecloud.objectstorage.object.create"],
    "data" : {
      "additionalDetails" : {
        "bucketName" : [oci_objectstorage_bucket.tams_temp_bucket.name],
        "objectName" : [{
          "prefix" : "chunks/"
        }]
      }
    }
  })

  actions {
    actions {
      action_type = "FAAS"
      is_enabled  = true

      function_id = oci_functions_function.video_analyzer.id
      description = "Trigger video analysis for chunks"
    }

    actions {
      action_type = "ONS"
      is_enabled  = true

      topic_id    = oci_ons_notification_topic.tams_processing_topic.id
      description = "Notify when chunks are created"
    }
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

# Function for handling chunk completion notifications
resource "oci_functions_function" "chunk_notifier" {
  application_id     = oci_functions_application.tams_functions_app.id
  display_name       = "chunk-notifier"
  image              = "${var.region}.ocir.io/${data.oci_objectstorage_namespace.ns.namespace}/${var.project_name}/chunk-notifier:latest"
  memory_in_mbs      = 256
  timeout_in_seconds = 30

  config = {
    DB_HOST      = oci_core_instance.tams_database.private_ip
    DB_PASSWORD  = var.db_admin_password
    DB_NAME      = "tamsdb"
    API_ENDPOINT = "http://${oci_load_balancer.tams_load_balancer.ip_address_details[0].ip_address}/api/v1"
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
    Type        = "NotificationHandler"
  }
}

# Dead Letter Queue for failed events
resource "oci_streaming_stream" "failed_events_stream" {
  name               = "${var.project_name}-failed-events"
  compartment_id     = var.compartment_ocid
  partitions         = 1
  retention_in_hours = 24

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

# Service connector for failed function invocations
resource "oci_sch_service_connector" "function_failures" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.project_name}-function-failures"
  description    = "Captures failed function invocations"

  source {
    kind = "logging"

    log_sources {
      compartment_id = var.compartment_ocid
      log_group_id   = oci_logging_log_group.functions_log_group.id
      log_id         = oci_logging_log.function_errors_log.id
    }
  }

  target {
    kind           = "streaming"
    compartment_id = var.compartment_ocid
    stream_id      = oci_streaming_stream.failed_events_stream.id
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

# Logging configuration for Functions
resource "oci_logging_log_group" "functions_log_group" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.project_name}-functions-logs"
  description    = "Log group for TAMS Functions"

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

resource "oci_logging_log" "function_errors_log" {
  display_name       = "${var.project_name}-function-errors"
  log_group_id       = oci_logging_log_group.functions_log_group.id
  log_type           = "SERVICE"
  retention_duration = 30

  configuration {
    source {
      category    = "invoke"
      resource    = oci_functions_application.tams_functions_app.id
      service     = "functions"
      source_type = "OCISERVICE"
    }

    compartment_id = var.compartment_ocid
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

# Monitoring and Alarms - Removed for POC