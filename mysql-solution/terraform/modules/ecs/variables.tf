variable "service_name" {
  type        = string
  description = "Base name for ECS resources"
}

variable "image" {
  type        = string
  description = "ECR image URI (with tag)"
}

variable "container_port" {
  type        = number
  description = "Port your app listens on"
}

variable "subnet_ids" {
  type        = list(string)
  description = "Subnets for FARGATE tasks"
}

variable "security_group_ids" {
  type        = list(string)
  description = "SGs for FARGATE tasks"
}

variable "execution_role_arn" {
  type        = string
  description = "ECS Task Execution Role ARN"
}

variable "task_role_arn" {
  type        = string
  description = "IAM Role ARN for app permissions"
}

variable "log_group_name" {
  type        = string
  description = "CloudWatch log group name"
}

variable "ecs_count" {
  type        = number
  default     = 1
  description = "Desired Fargate task count"
}

variable "region" {
  type        = string
  description = "AWS region (for awslogs driver)"
}

variable "cpu" {
  type        = string
  default     = "256"
  # default     = "512"
  description = "vCPU units"
}

variable "memory" {
  type        = string
  default     = "512"
  # default     = "1024"
  description = "Memory (MiB)"
}

variable "target_group_arn" {
  description = "ARN of the target group for the ECS service"
  type        = string
}

variable "db_instance_address" {
  description = "Address of the database instance"
  type        = string 
}

variable "db_port" {
  description = "Port of the database instance"
  type        = string 
  default     = "3306"
}

variable "db_username" {
  description = "database username"
  type        = string
  default     = "admin"
}

variable "db_password" {
  description = "database password"
  type        = string 
}

variable "db_name" {
  description = "database name"
  type        = string 
}
