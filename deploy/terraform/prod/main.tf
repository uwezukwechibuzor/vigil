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
  # backend "s3" {
  #   bucket  = "vigil-tf-state-prod"
  #   key     = "prod/terraform.tfstate"
  #   region  = var.region
  #   encrypt = true
  # }
}

provider "aws" {
  region = var.region
  default_tags {
    tags = {
      Environment = "prod"
      Project     = "vigil"
      ManagedBy   = "terraform"
    }
  }
}

locals {
  cluster_name = "vigil-prod"
  vpc_cidr     = "10.20.0.0/16"
}

data "aws_availability_zones" "available" { state = "available" }

resource "aws_vpc" "this" {
  cidr_block           = local.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support   = true
}

resource "aws_subnet" "private" {
  count             = 3
  vpc_id            = aws_vpc.this.id
  cidr_block        = cidrsubnet(local.vpc_cidr, 8, count.index)
  availability_zone = data.aws_availability_zones.available.names[count.index]
}

resource "aws_subnet" "public" {
  count                   = 3
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
  count          = 3
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

# --- EKS (3-AZ, production-sized) ---
module "eks" {
  source = "../modules/eks"

  cluster_name = local.cluster_name
  region       = var.region
  vpc_id       = aws_vpc.this.id
  subnet_ids   = aws_subnet.private[*].id

  node_groups = {
    general = {
      instance_types = ["m6g.large"]
      min_size       = 3
      max_size       = 20
      desired_size   = 6
    }
    compute = {
      instance_types = ["c6g.xlarge"]
      min_size       = 0
      max_size       = 10
      desired_size   = 2
    }
  }
}

# --- RDS Postgres (Multi-AZ) ---
module "rds" {
  source = "../modules/rds"

  identifier                 = "vigil-prod"
  instance_class             = "db.r7g.large"
  allocated_storage          = 100
  vpc_id                     = aws_vpc.this.id
  subnet_ids                 = aws_subnet.private[*].id
  multi_az                   = true
  backup_retention_days      = 30
  deletion_protection        = true
  allowed_security_group_ids = []
}

# --- ElastiCache Redis (cluster mode) ---
module "redis" {
  source = "../modules/elasticache"

  cluster_id                 = "vigil-prod"
  node_type                  = "cache.r7g.large"
  num_cache_nodes            = 3
  vpc_id                     = aws_vpc.this.id
  subnet_ids                 = aws_subnet.private[*].id
  allowed_security_group_ids = []
}

# --- S3 backups ---
module "backups" {
  source = "../modules/s3"

  bucket_name    = "vigil-backups-prod-${var.aws_account_id}"
  environment    = "prod"
  lifecycle_days = 90
}
