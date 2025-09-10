resource "oci_core_vcn" "tams_vcn" {
  compartment_id = var.compartment_ocid
  cidr_blocks    = ["10.0.0.0/16"]
  display_name   = "${var.project_name}-vcn"
  dns_label      = "tamsvcn"

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

resource "oci_core_internet_gateway" "tams_internet_gateway" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.tams_vcn.id
  display_name   = "${var.project_name}-igw"
  enabled        = true

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

resource "oci_core_nat_gateway" "tams_nat_gateway" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.tams_vcn.id
  display_name   = "${var.project_name}-nat"

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

resource "oci_core_service_gateway" "tams_service_gateway" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.tams_vcn.id
  display_name   = "${var.project_name}-sgw"

  services {
    service_id = data.oci_core_services.all_services.services[0].id
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

data "oci_core_services" "all_services" {
  filter {
    name   = "name"
    values = ["All .* Services In Oracle Services Network"]
    regex  = true
  }
}

resource "oci_core_subnet" "tams_public_subnet" {
  compartment_id    = var.compartment_ocid
  vcn_id            = oci_core_vcn.tams_vcn.id
  cidr_block        = "10.0.1.0/24"
  display_name      = "${var.project_name}-public-subnet"
  dns_label         = "public"
  route_table_id    = oci_core_route_table.tams_public_route_table.id
  security_list_ids = [oci_core_security_list.tams_public_security_list.id]

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
    Type        = "public"
  }
}

resource "oci_core_subnet" "tams_private_subnet" {
  compartment_id             = var.compartment_ocid
  vcn_id                     = oci_core_vcn.tams_vcn.id
  cidr_block                 = "10.0.2.0/24"
  display_name               = "${var.project_name}-private-subnet"
  dns_label                  = "private"
  prohibit_public_ip_on_vnic = true
  route_table_id             = oci_core_route_table.tams_private_route_table.id
  security_list_ids          = [oci_core_security_list.tams_private_security_list.id]

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
    Type        = "private"
  }
}

resource "oci_core_subnet" "tams_db_subnet" {
  compartment_id             = var.compartment_ocid
  vcn_id                     = oci_core_vcn.tams_vcn.id
  cidr_block                 = "10.0.3.0/24"
  display_name               = "${var.project_name}-db-subnet"
  dns_label                  = "database"
  prohibit_public_ip_on_vnic = true
  route_table_id             = oci_core_route_table.tams_private_route_table.id
  security_list_ids          = [oci_core_security_list.tams_db_security_list.id]

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
    Type        = "database"
  }
}

resource "oci_core_route_table" "tams_public_route_table" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.tams_vcn.id
  display_name   = "${var.project_name}-public-rt"

  route_rules {
    network_entity_id = oci_core_internet_gateway.tams_internet_gateway.id
    destination       = "0.0.0.0/0"
    destination_type  = "CIDR_BLOCK"
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

resource "oci_core_route_table" "tams_private_route_table" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.tams_vcn.id
  display_name   = "${var.project_name}-private-rt"

  route_rules {
    network_entity_id = oci_core_nat_gateway.tams_nat_gateway.id
    destination       = "0.0.0.0/0"
    destination_type  = "CIDR_BLOCK"
  }

  route_rules {
    network_entity_id = oci_core_service_gateway.tams_service_gateway.id
    destination       = data.oci_core_services.all_services.services[0].cidr_block
    destination_type  = "SERVICE_CIDR_BLOCK"
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

resource "oci_core_security_list" "tams_public_security_list" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.tams_vcn.id
  display_name   = "${var.project_name}-public-sl"

  ingress_security_rules {
    protocol    = "6"
    source      = "0.0.0.0/0"
    description = "HTTPS traffic for Web UI"

    tcp_options {
      min = 443
      max = 443
    }
  }

  ingress_security_rules {
    protocol    = "6"
    source      = "0.0.0.0/0"
    description = "HTTP traffic (redirect to HTTPS)"

    tcp_options {
      min = 80
      max = 80
    }
  }

  ingress_security_rules {
    protocol    = "6"
    source      = "0.0.0.0/0"
    description = "SSH for administration (consider restricting source IP)"

    tcp_options {
      min = 22
      max = 22
    }
  }

  egress_security_rules {
    protocol    = "all"
    destination = "0.0.0.0/0"
    description = "Allow all outbound traffic"
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

resource "oci_core_security_list" "tams_private_security_list" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.tams_vcn.id
  display_name   = "${var.project_name}-private-sl"

  ingress_security_rules {
    protocol    = "6"
    source      = "10.0.1.0/24"
    description = "TAMS API access ONLY from public subnet (load balancer)"

    tcp_options {
      min = 8080
      max = 8080
    }
  }

  ingress_security_rules {
    protocol    = "6"
    source      = "10.0.1.0/24"
    description = "SSH from bastion/public subnet only"

    tcp_options {
      min = 22
      max = 22
    }
  }

  ingress_security_rules {
    protocol    = "1"
    source      = "10.0.0.0/16"
    description = "ICMP for internal health checks"
  }

  egress_security_rules {
    protocol    = "all"
    destination = "0.0.0.0/0"
    description = "Allow all outbound traffic"
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

resource "oci_core_security_list" "tams_db_security_list" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.tams_vcn.id
  display_name   = "${var.project_name}-db-sl"

  ingress_security_rules {
    protocol = "6"
    source   = "10.0.2.0/24"

    tcp_options {
      min = 5432
      max = 5432
    }
  }

  egress_security_rules {
    protocol    = "6"
    destination = "0.0.0.0/0"

    tcp_options {
      min = 443
      max = 443
    }
  }

  freeform_tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}