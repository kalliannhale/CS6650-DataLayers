variable "worker_ecr_repository_name" {
  description = "The name of the ECR repository for the worker"
  type        = string
  default     = "ecr_worker"
}
variable "repository_name" {
  description = "The name of the ECR repository"
  type        = string
}