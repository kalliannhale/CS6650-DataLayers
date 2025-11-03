# Wire together four focused modules: network, ecr, logging, ecs.

module "network" {
  source         = "./modules/network"
  service_name   = var.service_name
  container_port = var.container_port
}

module "ecr" {
  source          = "./modules/ecr"
  repository_name = var.ecr_repository_name
}

module "logging" {
  source            = "./modules/logging"
  service_name      = var.service_name
  retention_in_days = var.log_retention_days
}

# Reuse an existing IAM role for ECS tasks
data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

module "ecs" {
  source              = "./modules/ecs"
  service_name        = var.service_name
  image               = "${module.ecr.repository_url}:latest"
  container_port      = var.container_port
  subnet_ids          = module.network.subnet_ids
  security_group_ids  = [module.network.security_group_id]
  execution_role_arn  = data.aws_iam_role.lab_role.arn
  task_role_arn       = data.aws_iam_role.lab_role.arn
  log_group_name      = module.logging.log_group_name
  ecs_count           = var.ecs_count
  region              = var.aws_region
  target_group_arn    = aws_lb_target_group.app_tg.arn
  db_instance_address = module.rds.db_instance_address
  db_port             = tostring(module.rds.db_instance_port)
  db_username         = module.rds.db_username
  db_password         = module.rds.db_password
  db_name             = module.rds.db_name

  depends_on = [module.rds]
}

module "rds" {
  source              = "./modules/rds"
  vpc_id    = module.network.vpc_id
  ecs_sg_id = module.network.security_group_id
  db_identifier   = var.db_idenfifier
}

// Build & push the Go app image into ECR
resource "docker_image" "app" {
  # Use the URL from the ecr module, and tag it "latest"
  name = "${module.ecr.repository_url}:latest"

  build {
    # relative path from terraform/ → src/
    context = "../src"
    # Dockerfile defaults to "Dockerfile" in that context
  }
}

resource "docker_registry_image" "app" {
  # this will push :latest → ECR
  name = docker_image.app.name
}

resource "aws_lb" "app" {
  name               = "${var.service_name}-alb"
  load_balancer_type = "application"
  subnets            = module.network.subnet_ids
  security_groups    = [module.network.security_group_id]
}

resource "aws_lb_target_group" "app_tg" {
  name        = "${var.service_name}-tg"
  port        = var.container_port
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = module.network.vpc_id

  health_check {
    path                = "/health"
    interval            = 30
    healthy_threshold   = 2
  }
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.app.arn
  port              = 8080
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.app_tg.arn
  }
}