
# Attain availablity zones
data "aws_availability_zones" "this" {
  state = "available"
}

# Create a private subnet to place RDS MySQR instance
resource "aws_subnet" "private-1" {
  vpc_id            = var.vpc_id
  cidr_block        = var.private_subnet_cidr1
  availability_zone = data.aws_availability_zones.this.names[0]

  tags = { 
    Name = "${var.db_identifier}-private-subnet-1" 
  }
}

# Create the second private subset for RDS subnet-group requirement
resource "aws_subnet" "private-2" {
  vpc_id            = var.vpc_id
  cidr_block        = var.private_subnet_cidr2
  availability_zone = data.aws_availability_zones.this.names[1]

  tags = { 
    Name = "${var.db_identifier}-private-subnet-2" 
  }
}

# Create a private route table
resource "aws_route_table" "private" {
  vpc_id = var.vpc_id
  tags   = { 
    Name = "${var.db_identifier}-private-rt" 
  }
}

# Associate the private subnet with route table
resource "aws_route_table_association" "private-1" {
  subnet_id      = aws_subnet.private-1.id
  route_table_id = aws_route_table.private.id
}

resource "aws_route_table_association" "private-2" {
  subnet_id      = aws_subnet.private-2.id
  route_table_id = aws_route_table.private.id
}

# Create a security group for private subnet
resource "aws_security_group" "rds_sg" {
  name        = "${var.db_identifier}-rds-sg"
  description = "Security group for RDS MySQL instance"
  vpc_id      = var.vpc_id

  ingress {
    from_port   = 3306
    to_port     = 3306
    protocol    = "tcp"
    security_groups = [var.ecs_sg_id]
    description = "Allow MySQL access from ECS task"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound (limited by subnet routing)"
  }
}

# Create a DB Subnet Group 
resource "aws_db_subnet_group" "this" {
    name       = "${var.db_identifier}-subnet-group"
    subnet_ids = [aws_subnet.private-1.id, aws_subnet.private-2.id]
}

# Create random password
resource "random_password" "db_password" {
  length  = 16
  special = true
  override_special = "!#$%&*()-_=+[]{}<>:?"
}

# Create a RDS MySQL database
resource "aws_db_instance" "mysql" {
  identifier = var.db_identifier
  
  # engine configuration
  engine         = "mysql"
  engine_version = "8.0"
  instance_class = "db.t3.micro"

  # storage configuration
  allocated_storage = 20
  storage_type      = "gp2"
  storage_encrypted = true

  # database configuration
  username = "admin"
  password = random_password.db_password.result
  parameter_group_name = "default.mysql8.0"
  db_name = "mydb"
  
  # network configuration
  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [aws_security_group.rds_sg.id]
  publicly_accessible    = false
  multi_az = false

  # backup configuration
  backup_retention_period = 7

  # deletion confiuration
  skip_final_snapshot = true
  deletion_protection = false
}