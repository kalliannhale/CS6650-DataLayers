variable "vpc_id" {
  description = "VPC id for private subnet"
  type        = string
}

variable "private_subnet_cidr1" {
  description = "CIDR for private subnet"
  type        = string
  default     = "172.31.64.0/20"
}

variable "private_subnet_cidr2" {
  description = "CIDR for private subnet"
  type        = string
  default     = "172.31.80.0/20"
}

variable "ecs_sg_id" {
  description = "Security group ID for ECS tasks"
  type        = string
}

variable "db_identifier" {
  description = "Name for database"
  type        = string
}