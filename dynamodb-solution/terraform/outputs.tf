output "ecs_cluster_name" {
  description = "Name of the created ECS cluster"
  value       = module.ecs.cluster_name # This should point to "ecs"
}

output "ecs_service_name" {
  description = "Name of the running ECS service"
  value       = module.ecs.service_name # This should point to "ecs"
}

output "alb_dns_name" {
  description = "The DNS name of the Application Load Balancer"
  value       = aws_alb.this.dns_name
}

output "debug_docker_context_path" {
  value = abspath("${path.module}/../../dynamodb-solution")
}