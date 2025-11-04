
/*output "subnet_ids" {
  description = "IDs of the default VPC subnets"
  value       = data.aws_subnets.default.ids
}

output "security_group_id" {
  description = "Security group ID for ECS"
  value       = aws_security_group.this.id
}
*/


output "vpc_id" {
  description = "The default VPC ID"
  value       = data.aws_vpc.default.id
}

output "public_subnet_ids" {
  description = "IDs of the public subnets"
  value       = aws_subnet.public[*].id
}

output "private_subnet_ids" {
  description = "IDs of the private subnets"
  value       = aws_subnet.private[*].id
}

output "alb_security_group_id" {
  description = "Security group ID for the ALB"
  value       = aws_security_group.alb.id
}

output "ecs_security_group_id" {
  description = "Security group ID for the ECS Tasks"
  value       = aws_security_group.ecs_tasks.id
}

