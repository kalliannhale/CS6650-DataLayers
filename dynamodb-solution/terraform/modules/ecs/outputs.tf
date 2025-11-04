output "cluster_name" {
  description = "ECS cluster name"
  value       = aws_ecs_cluster.this.name
}

output "service_name" {
  description = "ECS service name"
  value       = aws_ecs_service.this.name
}


output "service_scalability_resource_id" {
  description = "The resource ID for auto-scaling"
  value       = "service/${aws_ecs_cluster.this.name}/${aws_ecs_service.this.name}"
}
