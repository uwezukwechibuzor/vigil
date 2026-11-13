terraform {
  required_version = ">= 1.6.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.0"
    }
  }
  # Uncomment to store state in S3
  # backend "s3" {
  #   bucket = "vigil-tf-state-staging"
  #   key    = "staging/terraform.tfstate"
  #   region = var.region
  #   encrypt = true
  # }
}

provider "aws" {
  region = var.region
  default_tags {
    tags = {
      Environment = "staging"
      Project     = "vigil"
      ManagedBy   = "terraform"
    }
  }
}

locals {
  cluster_name = "vigil-staging"
  vpc_cidr     = "10.10.0.0/16"
}

# --- Networking (simplified: rely on existing VPC or create minimal one) ---
data "aws_availability_zones" "available" { state = "available" }

resource "aws_vpc" "this" {
  cidr_block           = local.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support   = true
}

resource "aws_subnet" "private" {
  count             = 2
  vpc_id            = aws_vpc.this.id
  cidr_block        = cidrsubnet(local.vpc_cidr, 8, count.index)
  availability_zone = data.aws_availability_zones.available.names[count.index]
}

resource "aws_subnet" "public" {
  count                   = 2
  vpc_id                  = aws_vpc.this.id
  cidr_block              = cidrsubnet(local.vpc_cidr, 8, count.index + 100)
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = true
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.this.id
  }
}

resource "aws_route_table_association" "public" {
  count          = 2
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

# --- EKS ---
module "eks" {
  source = "../modules/eks"

  cluster_name = local.cluster_name
  region       = var.region
  vpc_id       = aws_vpc.this.id
  subnet_ids   = aws_subnet.private[*].id

  node_groups = {
    general = {
      instance_types = ["t3.medium"]
      min_size       = 1
      max_size       = 5
      desired_size   = 2
    }
  }
}

# --- RDS Postgres ---
module "rds" {
  source = "../modules/rds"

  identifier                 = "vigil-staging"
  vpc_id                     = aws_vpc.this.id
  subnet_ids                 = aws_subnet.private[*].id
  multi_az                   = false
  backup_retention_days      = 3
  deletion_protection        = false
  allowed_security_group_ids = []
}

# --- ElastiCache Redis ---
module "redis" {
  source = "../modules/elasticache"

  cluster_id                 = "vigil-staging"
  vpc_id                     = aws_vpc.this.id
  subnet_ids                 = aws_subnet.private[*].id
  num_cache_nodes            = 1
  allowed_security_group_ids = []
}

# --- S3 backups ---
module "backups" {
  source = "../modules/s3"

  bucket_name   = "vigil-backups-staging-${var.aws_account_id}"
  environment   = "staging"
  lifecycle_days = 30
}
