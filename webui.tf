resource "oci_core_instance" "tams_webui" {
  availability_domain = local.availability_domain
  compartment_id      = var.compartment_ocid
  shape               = var.instance_shape

  shape_config {
    ocpus         = 1
    memory_in_gbs = 4
  }

  display_name = "${var.project_name}-webui"

  create_vnic_details {
    subnet_id        = oci_core_subnet.tams_public_subnet.id
    display_name     = "${var.project_name}-webui-vnic"
    assign_public_ip = true
    hostname_label   = "tamswebui"
  }

  source_details {
    source_type = "image"
    source_id   = data.oci_core_images.ubuntu.images[0].id
  }

  metadata = {
    ssh_authorized_keys = file(var.ssh_public_key_path)
    user_data = base64encode(templatefile("${path.module}/scripts/init_webui.sh", {
      API_URL          = "http://${oci_load_balancer.tams_load_balancer.ip_address_details[0].ip_address}"
      INTERNAL_API_URL = "http://10.0.2.10:8080"
      WEB_DOMAIN       = var.web_domain
    }))
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
    Role        = "WebUI"
    Type        = "Frontend"
  }
}

resource "oci_load_balancer_backend_set" "tams_webui_backend_set" {
  load_balancer_id = oci_load_balancer.tams_load_balancer.id
  name             = "${var.project_name}-webui-backend-set"
  policy           = "ROUND_ROBIN"

  health_checker {
    protocol          = "HTTP"
    port              = 8090
    url_path          = "/health"
    return_code       = 200
    interval_ms       = 10000
    timeout_in_millis = 3000
    retries           = 3
  }

  session_persistence_configuration {
    cookie_name      = "tams-webui-session"
    disable_fallback = false
  }
}

resource "oci_load_balancer_backend" "tams_webui_backend" {
  load_balancer_id = oci_load_balancer.tams_load_balancer.id
  backendset_name  = oci_load_balancer_backend_set.tams_webui_backend_set.name
  ip_address       = oci_core_instance.tams_webui.private_ip
  port             = 8090
}

resource "oci_load_balancer_listener" "tams_main_listener" {
  load_balancer_id         = oci_load_balancer.tams_load_balancer.id
  name                     = "${var.project_name}-main-listener"
  default_backend_set_name = oci_load_balancer_backend_set.tams_webui_backend_set.name
  port                     = 443
  protocol                 = "HTTP"
  path_route_set_name      = oci_load_balancer_path_route_set.tams_routes.name

  connection_configuration {
    idle_timeout_in_seconds = 60
  }

  ssl_configuration {
    certificate_name        = oci_load_balancer_certificate.tams_certificate.certificate_name
    verify_peer_certificate = false
  }
}

resource "oci_load_balancer_path_route_set" "tams_routes" {
  load_balancer_id = oci_load_balancer.tams_load_balancer.id
  name             = "${var.project_name}-routes"

  path_routes {
    path             = "/api/*"
    backend_set_name = oci_load_balancer_backend_set.tams_api_backend_set.name

    path_match_type {
      match_type = "PREFIX_MATCH"
    }
  }

  path_routes {
    path             = "/"
    backend_set_name = oci_load_balancer_backend_set.tams_webui_backend_set.name

    path_match_type {
      match_type = "PREFIX_MATCH"
    }
  }
}

resource "oci_load_balancer_rule_set" "security_headers" {
  load_balancer_id = oci_load_balancer.tams_load_balancer.id
  name             = "${var.project_name}_security_headers"

  items {
    action = "ADD_HTTP_RESPONSE_HEADER"
    header = "Strict-Transport-Security"
    value  = "max-age=31536000; includeSubDomains"
  }

  items {
    action = "ADD_HTTP_RESPONSE_HEADER"
    header = "X-Content-Type-Options"
    value  = "nosniff"
  }

  items {
    action = "ADD_HTTP_RESPONSE_HEADER"
    header = "X-Frame-Options"
    value  = "SAMEORIGIN"
  }

  items {
    action = "ADD_HTTP_RESPONSE_HEADER"
    header = "Content-Security-Policy"
    value  = "default-src 'self' https:; script-src 'self' 'unsafe-inline' https://cdn.tailwindcss.com https://cdnjs.cloudflare.com; style-src 'self' 'unsafe-inline' https://cdnjs.cloudflare.com"
  }
}

resource "oci_core_network_security_group" "tams_webui_nsg" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.tams_vcn.id
  display_name   = "${var.project_name}-webui-nsg"

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

resource "oci_core_network_security_group_security_rule" "webui_ingress_https" {
  network_security_group_id = oci_core_network_security_group.tams_webui_nsg.id
  direction                 = "INGRESS"
  protocol                  = "6"
  description               = "Allow HTTPS from Internet"

  source      = "0.0.0.0/0"
  source_type = "CIDR_BLOCK"

  tcp_options {
    destination_port_range {
      min = 443
      max = 443
    }
  }
}

resource "oci_core_network_security_group_security_rule" "webui_ingress_http" {
  network_security_group_id = oci_core_network_security_group.tams_webui_nsg.id
  direction                 = "INGRESS"
  protocol                  = "6"
  description               = "Allow HTTP for redirect to HTTPS"

  source      = "0.0.0.0/0"
  source_type = "CIDR_BLOCK"

  tcp_options {
    destination_port_range {
      min = 80
      max = 80
    }
  }
}

resource "oci_core_network_security_group" "tams_api_nsg" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.tams_vcn.id
  display_name   = "${var.project_name}-api-nsg"

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

resource "oci_core_network_security_group_security_rule" "api_ingress_internal" {
  network_security_group_id = oci_core_network_security_group.tams_api_nsg.id
  direction                 = "INGRESS"
  protocol                  = "6"
  description               = "Allow API access ONLY from VCN internal"

  source      = "10.0.0.0/16"
  source_type = "CIDR_BLOCK"

  tcp_options {
    destination_port_range {
      min = 8080
      max = 8080
    }
  }
}

resource "oci_core_network_security_group_security_rule" "api_deny_external" {
  network_security_group_id = oci_core_network_security_group.tams_api_nsg.id
  direction                 = "INGRESS"
  protocol                  = "all"
  description               = "Explicitly deny all external access"

  source      = "0.0.0.0/0"
  source_type = "CIDR_BLOCK"
  stateless   = true
}