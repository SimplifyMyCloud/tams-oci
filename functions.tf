resource "oci_functions_application" "tams_functions_app" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.project_name}-functions-app"
  subnet_ids     = [oci_core_subnet.tams_private_subnet.id]

  config = {
    OCI_NAMESPACE          = data.oci_objectstorage_namespace.ns.namespace
    OCI_REGION            = var.region
    MEDIA_BUCKET          = oci_objectstorage_bucket.tams_media_bucket.name
    ARCHIVE_BUCKET        = oci_objectstorage_bucket.tams_archive_bucket.name
    TEMP_BUCKET           = oci_objectstorage_bucket.tams_temp_bucket.name
    TARGET_DURATION       = "60"
    SCENE_THRESHOLD       = "0.3"
    MAX_DEVIATION         = "0.2"
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

resource "oci_functions_function" "video_chunker" {
  application_id = oci_functions_application.tams_functions_app.id
  display_name   = "video-chunker"
  image          = "${var.region}.ocir.io/${data.oci_objectstorage_namespace.ns.namespace}/${var.project_name}/video-chunker:latest"
  memory_in_mbs  = 2048
  timeout_in_seconds = 300

  config = {
    TARGET_DURATION = "60"
    SCENE_THRESHOLD = "0.3"
    MAX_DEVIATION   = "0.2"
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
    Type        = "VideoProcessing"
  }
}

resource "oci_functions_function" "video_processor" {
  application_id = oci_functions_application.tams_functions_app.id
  display_name   = "video-processor"
  image          = "${var.region}.ocir.io/${data.oci_objectstorage_namespace.ns.namespace}/${var.project_name}/video-processor:latest"
  memory_in_mbs  = 512
  timeout_in_seconds = 60

  config = {
    OCI_NAMESPACE     = data.oci_objectstorage_namespace.ns.namespace
    OCI_REGION        = var.region
    MEDIA_BUCKET      = oci_objectstorage_bucket.tams_media_bucket.name
    TEMP_BUCKET       = oci_objectstorage_bucket.tams_temp_bucket.name
    CHUNKER_ENDPOINT  = "https://${oci_apigateway_gateway.tams_api_gateway.hostname}/tams/chunk-video"
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
    Type        = "EventProcessor"
  }
}

resource "oci_functions_function" "video_analyzer" {
  application_id = oci_functions_application.tams_functions_app.id
  display_name   = "video-analyzer"
  image          = "${var.region}.ocir.io/${data.oci_objectstorage_namespace.ns.namespace}/${var.project_name}/video-analyzer:latest"
  memory_in_mbs  = 1024
  timeout_in_seconds = 120

  config = {
    OCI_COMPARTMENT_ID = var.compartment_ocid
    TAMS_API_ENDPOINT  = "http://${oci_load_balancer.tams_load_balancer.ip_address_details[0].ip_address}/api/v1"
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
    Type        = "VideoAnalysis"
  }
}

# IAM policy for Functions to access Object Storage
resource "oci_identity_policy" "functions_policy" {
  compartment_id = var.compartment_ocid
  name           = "${var.project_name}-functions-policy"
  description    = "Policy for TAMS Functions to access resources"

  statements = [
    "Allow service FaaS to read repos in compartment id ${var.compartment_ocid}",
    "Allow service FaaS to use virtual-network-family in compartment id ${var.compartment_ocid}",
    "Allow dynamic-group ${oci_identity_dynamic_group.functions_dynamic_group.name} to manage objects in compartment id ${var.compartment_ocid}",
    "Allow dynamic-group ${oci_identity_dynamic_group.functions_dynamic_group.name} to manage buckets in compartment id ${var.compartment_ocid}",
  ]

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

resource "oci_identity_dynamic_group" "functions_dynamic_group" {
  compartment_id = var.tenancy_ocid
  name           = "${var.project_name}-functions-dg"
  description    = "Dynamic group for TAMS Functions"
  
  matching_rule = "ALL {resource.type='fnfunc', resource.compartment.id='${var.compartment_ocid}'}"

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

# API Gateway for Functions
resource "oci_apigateway_gateway" "tams_api_gateway" {
  compartment_id = var.compartment_ocid
  endpoint_type  = "PUBLIC"
  subnet_id      = oci_core_subnet.tams_public_subnet.id
  display_name   = "${var.project_name}-api-gateway"

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

resource "oci_apigateway_deployment" "functions_deployment" {
  compartment_id = var.compartment_ocid
  gateway_id     = oci_apigateway_gateway.tams_api_gateway.id
  display_name   = "${var.project_name}-functions-deployment"
  path_prefix    = "/tams"

  specification {
    routes {
      path    = "/chunk-video"
      methods = ["POST"]

      backend {
        type = "ORACLE_FUNCTIONS_BACKEND"
        function_id = oci_functions_function.video_chunker.id
      }

      request_policies {
        body_validation {
          content {
            media_type = "application/json"
            validation_type = "DISABLED"
          }
        }
      }

      response_policies {
        header_transformations {
          set_headers {
            items {
              name = "Access-Control-Allow-Origin"
              values = ["*"]
            }
          }
        }
      }
    }

    routes {
      path    = "/health"
      methods = ["GET"]

      backend {
        type = "HTTP_BACKEND"
        url  = "https://httpbin.org/status/200"
      }
    }
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

# Container Registry for Functions
resource "oci_artifacts_container_repository" "video_chunker_repo" {
  compartment_id   = var.compartment_ocid
  display_name     = "${var.project_name}/video-chunker"
  is_immutable     = false
  is_public        = false

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}