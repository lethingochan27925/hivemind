data "aws_availability_zones" "available" {
  state = "available"
}

locals {
  azs = slice(data.aws_availability_zones.available.names, 0, var.az_count)

  # CIDR blocks cho từng subnet
  public_subnets  = [for i, az in local.azs : cidrsubnet(var.vpc_cidr, 8, i)]
  private_subnets = [for i, az in local.azs : cidrsubnet(var.vpc_cidr, 8, i + 10)]

  common_tags = {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "terraform"
    Module      = "networking"
  }
}

# ── VPC ───────────────────────────────────────────────────────────────────────
resource "aws_vpc" "main" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = merge(local.common_tags, { Name = "${var.project}-${var.environment}-vpc" })
}

# ── Internet Gateway ──────────────────────────────────────────────────────────
resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id
  tags   = merge(local.common_tags, { Name = "${var.project}-${var.environment}-igw" })
}

# ── Public Subnets ────────────────────────────────────────────────────────────
resource "aws_subnet" "public" {
  count             = var.az_count
  vpc_id            = aws_vpc.main.id
  cidr_block        = local.public_subnets[count.index]
  availability_zone = local.azs[count.index]

  map_public_ip_on_launch = true

  tags = merge(local.common_tags, {
    Name = "${var.project}-${var.environment}-public-${local.azs[count.index]}"
    Tier = "public"
    # EKS tag — required cho EKS nhận diện subnet
    "kubernetes.io/role/elb" = "1"
  })
}

# ── Private Subnets ───────────────────────────────────────────────────────────
resource "aws_subnet" "private" {
  count             = var.az_count
  vpc_id            = aws_vpc.main.id
  cidr_block        = local.private_subnets[count.index]
  availability_zone = local.azs[count.index]

  tags = merge(local.common_tags, {
    Name = "${var.project}-${var.environment}-private-${local.azs[count.index]}"
    Tier = "private"
    # EKS tag — required cho internal load balancer
    "kubernetes.io/role/internal-elb" = "1"
  })
}

# ── Elastic IPs cho NAT Gateway ───────────────────────────────────────────────
resource "aws_eip" "nat" {
  count  = var.az_count
  domain = "vpc"
  tags   = merge(local.common_tags, { Name = "${var.project}-${var.environment}-eip-${count.index}" })

  depends_on = [aws_internet_gateway.main]
}

# ── NAT Gateway (một per AZ — high availability) ──────────────────────────────
resource "aws_nat_gateway" "main" {
  count         = var.az_count
  allocation_id = aws_eip.nat[count.index].id
  subnet_id     = aws_subnet.public[count.index].id

  tags = merge(local.common_tags, {
    Name = "${var.project}-${var.environment}-nat-${local.azs[count.index]}"
  })

  depends_on = [aws_internet_gateway.main]
}

# ── Route Table — Public ──────────────────────────────────────────────────────
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = merge(local.common_tags, { Name = "${var.project}-${var.environment}-rt-public" })
}

resource "aws_route_table_association" "public" {
  count          = var.az_count
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

# ── Route Table — Private (một per AZ) ───────────────────────────────────────
resource "aws_route_table" "private" {
  count  = var.az_count
  vpc_id = aws_vpc.main.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.main[count.index].id
  }

  tags = merge(local.common_tags, {
    Name = "${var.project}-${var.environment}-rt-private-${local.azs[count.index]}"
  })
}

resource "aws_route_table_association" "private" {
  count          = var.az_count
  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private[count.index].id
}

# ── Security Group — EKS Nodes ────────────────────────────────────────────────
resource "aws_security_group" "eks_nodes" {
  name        = "${var.project}-${var.environment}-eks-nodes-sg"
  description = "Security group for EKS worker nodes"
  vpc_id      = aws_vpc.main.id

  # Nodes giao tiếp với nhau
  ingress {
    description = "Node to node communication"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    self        = true
  }

  # Control plane giao tiếp với nodes
  ingress {
    description     = "EKS control plane to nodes"
    from_port       = 1025
    to_port         = 65535
    protocol        = "tcp"
    security_groups = [aws_security_group.eks_control_plane.id]
  }

  # Egress — ra CockroachDB, Bedrock, MCP
  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.common_tags, { Name = "${var.project}-${var.environment}-eks-nodes-sg" })
}

# ── Security Group — EKS Control Plane ───────────────────────────────────────
resource "aws_security_group" "eks_control_plane" {
  name        = "${var.project}-${var.environment}-eks-cp-sg"
  description = "Security group for EKS control plane"
  vpc_id      = aws_vpc.main.id

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.common_tags, { Name = "${var.project}-${var.environment}-eks-cp-sg" })
}

# ── Security Group — Lambda ───────────────────────────────────────────────────
resource "aws_security_group" "lambda" {
  name        = "${var.project}-${var.environment}-lambda-sg"
  description = "Security group for Lambda functions (Dispatcher, Reaper)"
  vpc_id      = aws_vpc.main.id

  egress {
    description = "Allow all outbound (CockroachDB, Bedrock, MCP)"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.common_tags, { Name = "${var.project}-${var.environment}-lambda-sg" })
}
