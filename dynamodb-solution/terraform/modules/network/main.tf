
/*# Fetch the default VPC
data "aws_vpc" "default" {
  default = true
}

# List all subnets in that VPC
data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

# Create a security group to allow HTTP to your container port
resource "aws_security_group" "this" {
  name        = "${var.service_name}-sg"
  description = "Allow inbound on ${var.container_port}"
  vpc_id      = data.aws_vpc.default.id

  ingress {
    from_port   = var.container_port
    to_port     = var.container_port
    protocol    = "tcp"
    cidr_blocks = var.cidr_blocks
    description = "Allow HTTP traffic"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound"
  }
}*/



# Fetch the default VPC
data "aws_vpc" "default" {
  default = true
}

# Get all availability zones in the region
data "aws_availability_zones" "available" {
  state = "available"
}

# --- Subnets ---
# Create 2 public subnets for the ALB
resource "aws_subnet" "public" {
  count                   = 2
  vpc_id                  = data.aws_vpc.default.id
  # Use new CIDR blocks to avoid any conflicts
  cidr_block              = cidrsubnet(data.aws_vpc.default.cidr_block, 8, 180 + count.index) # e.g., 172.31.180.0/24
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = true # Public subnets get public IPs

  tags = {
    Name = "${var.service_name}-public-subnet-${count.index + 1}"
  }
}

# Create 2 private subnets for Fargate
resource "aws_subnet" "private" {
  count                   = 2
  vpc_id                  = data.aws_vpc.default.id
  # Use new CIDR blocks to avoid any conflicts
  cidr_block              = cidrsubnet(data.aws_vpc.default.cidr_block, 8, 190 + count.index) # e.g., 172.31.190.0/24
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = false # Private subnets DO NOT get public IPs

  tags = {
    Name = "${var.service_name}-private-subnet-${count.index + 1}"
  }
}

# --- Internet Gateway & Routing (for Public Subnets) ---
# Find the main route table, which is already public and has an internet route
data "aws_route_table" "main" {
  vpc_id = data.aws_vpc.default.id
  filter {
    name   = "association.main"
    values = ["true"]
  }
}

# Associate the public subnets with the main/public route table
resource "aws_route_table_association" "public" {
  count          = length(aws_subnet.public)
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = data.aws_route_table.main.id
}

# --- NAT Gateway & Routing (for Private Subnets) ---
# (Allows Fargate tasks to access the internet, e.g., to pull from ECR)
resource "aws_eip" "nat" {
  domain = "vpc"
}

resource "aws_nat_gateway" "this" {
  allocation_id = aws_eip.nat.id
  subnet_id     = aws_subnet.public[0].id # Put NAT GW in a public subnet
  tags          = { Name = "${var.service_name}-nat-gw" }
  # Depend on the main route table existing
  depends_on = [data.aws_route_table.main]
}

resource "aws_route_table" "private" {
  vpc_id = data.aws_vpc.default.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.this.id
  }

  tags = { Name = "${var.service_name}-private-rt" }
}

resource "aws_route_table_association" "private" {
  count          = length(aws_subnet.private)
  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private.id
}

resource "aws_vpc_endpoint" "dynamodb_gateway_endpoint" {
  service_name      = "com.amazonaws.${var.aws_region}.dynamodb"
  vpc_endpoint_type = "Gateway"
  
  # Uses the VPC ID found by the data source
  vpc_id = data.aws_vpc.default.id 

  # The gateway endpoint must update the private route table used by Fargate subnets
  route_table_ids = [aws_route_table.private.id]
  
  tags = {
    Name = "${var.service_name}-dynamodb-endpoint"
  }

  # Ensure this endpoint is created before the tasks deploy
  depends_on = [aws_route_table.private]
}

# --- Security Groups ---

# Security Group for the ALB ("Front Door")
# Allows public HTTP traffic
resource "aws_security_group" "alb" {
  name        = "${var.service_name}-alb-sg"
  description = "Allow inbound HTTP to ALB"
  vpc_id      = data.aws_vpc.default.id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow HTTP traffic from anywhere"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound"
  }
}

# Security Group for the ECS Fargate Tasks ("Kitchen")
# Allows traffic ONLY from the ALB
resource "aws_security_group" "ecs_tasks" {
  name        = "${var.service_name}-ecs-sg"
  description = "Allow inbound from ALB to ECS"
  vpc_id      = data.aws_vpc.default.id

  ingress {
    from_port   = var.container_port
    to_port     = var.container_port
    protocol    = "tcp"
    # CRITICAL: Only allow traffic from the ALB's security group
    security_groups = [aws_security_group.alb.id]
    description     = "Allow traffic from ALB"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound (for ECR, etc)"
  }
}

