# --- Base Modules (Network, ECR, Logging for API) ---
module "network" {
  source       = "./modules/network"
  service_name = var.service_name
  container_port = var.container_port
}

# Phase 1 ECR Repo
module "ecr" {
  source          = "./modules/ecr"
  repository_name = var.ecr_repository_name
}

# Phase 1 Logging
module "logging" {
  source            = "./modules/logging"
  service_name      = var.service_name
  retention_in_days = var.log_retention_days
}

/*
# --- HW7: Messaging Module (SNS & SQS) ---
# We don't need this for the shopping cart API
module "messaging" {
  source = "./modules/messaging"
}
*/

# This data source is used by your ECS module, so we keep it.
# This tells us your task role is named "LabRole".
data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

# --- API Service (ALB + ECS) ---
resource "aws_alb" "this" {
  name             = "${var.service_name}-alb"
  internal         = false
  load_balancer_type = "application"
  security_groups  = [module.network.alb_security_group_id]
  subnets          = module.network.public_subnet_ids
  tags = {
    Name = "${var.service_name}-alb"
  }
}

resource "aws_alb_target_group" "this" {
  name        = "${var.service_name}-tg"
  port        = 8080 # Your Go app runs on :8080
  protocol    = "HTTP"
  vpc_id      = module.network.vpc_id
  target_type = "ip"
  health_check {
    path     = "/health" # Your Go app has this
    protocol = "HTTP"
    interval = 30
  }
}

resource "aws_alb_listener" "http" {
  load_balancer_arn = aws_alb.this.arn
  port              = 80
  protocol          = "HTTP"
  default_action {
    type             = "forward"
    target_group_arn = aws_alb_target_group.this.arn
  }
}

module "dynamodb" {
  source = "./modules/dynamoDB"

  table_name = "shopping-carts-table"

  tags = {
    Assignment = "Homework-8-DynamoDB"
  }
}


module "ecs" {
  source             = "./modules/ecs"
  service_name       = var.service_name
  image              = docker_registry_image.app.name
  container_port     = var.container_port
  subnet_ids         = module.network.private_subnet_ids
  security_group_ids = [module.network.ecs_security_group_id]
  execution_role_arn = data.aws_iam_role.lab_role.arn
  task_role_arn      = data.aws_iam_role.lab_role.arn
  log_group_name     = module.logging.log_group_name
  region             = var.aws_region
  target_group_arn   = aws_alb_target_group.this.arn
  desired_count      = 1

  environment_variables = {
    "DATABASE_TYPE"       = "dynamodb" # For the "switcher" logic
    "DYNAMODB_TABLE_NAME" = module.dynamodb.table_name # From our new module
    "AWS_REGION"          = var.aws_region
  }
}


/*
# --- HW7: Worker Service ---
# We don't need this for the shopping cart API

# Worker Service ECR Repo
module "ecr_worker" {
  source          = "./modules/ecr"
  repository_name = var.worker_ecr_repository_name
}

# Worker Service Logging
module "logging_worker" {
  source            = "./modules/logging"
  service_name      = "async-worker" # New name for CloudWatch
  retention_in_days = var.log_retention_days
}

# Worker Service ECS Definition
module "ecs_worker" {
  source             = "./modules/ecs"
  service_name       = "async-worker"
  image              = docker_registry_image.worker_app.name
  container_port     = var.container_port # Not used, but required by module
  subnet_ids         = module.network.private_subnet_ids
  security_group_ids = [module.network.ecs_security_group_id]
  execution_role_arn = data.aws_iam_role.lab_role.arn
  task_role_arn      = data.aws_iam_role.lab_role.arn # MODIFIED: Use LabRole
  log_group_name     = module.logging_worker.log_group_name
  region             = var.aws_region
  desired_count      = 1 # Requirement: Start with 1 worker

  # NEW: Pass the SQS Queue URL
  environment_variables = {
    "SQS_QUEUE_URL" = module.messaging.sqs_queue_url
  }
}
*/

# --- Docker Build/Push ---

# API App (from src/)
# This is the *only* app we need to build for HW8
resource "docker_image" "app" {
  name = "${module.ecr.repository_url}:latest" 
  build {
    context    = "${path.module}/../../dynamodb-solution/src"
    dockerfile = "Dockerfile"
  }
}
resource "docker_registry_image" "app" {
  name       = docker_image.app.name
  depends_on = [docker_image.app]
}

/*
# --- HW7: Worker App Docker ---
# We don't need this

# NEW: Worker App (from worker/)
resource "docker_image" "worker_app" {
  name = "${module.ecr_worker.repository_url}:latest"
  build {
    context    = "../worker"
    dockerfile = "Dockerfile"
  }
}
resource "docker_registry_image" "worker_app" {
  name       = docker_image.worker_app.name
  depends_on = [module.ecr_worker]
}
*/

/*
# --- Part III: Lambda Function ---
# We don't need this for the shopping cart API

# 1. NEW: Build the Go binary using a null_resource
# This provisioner runs the Go build command locally
resource "null_resource" "build_lambda" {
  # This provisioner runs the Go build command
  provisioner "local-exec" {
    command     = "GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bootstrap main.go"
    working_dir = "../lambda"
  }

  # This forces the build to re-run if your Go code changes
  triggers = {
    go_mod = filemd5("../lambda/go.mod")
    go_src = filemd5("../lambda/main.go")
  }
}

# 2. Build and Zip the Lambda code
# This zips the *already-built* binary
data "archive_file" "lambda_zip" {
  type        = "zip"
  source_dir  = "../lambda"
  output_path = "${path.module}/lambda.zip"

  # CRITICAL: This data source must depend on the build finishing
  depends_on = [null_resource.build_lambda]
}

# 3. Define the Lambda function
resource "aws_lambda_function" "order_processor_lambda" {
  function_name = "order-processor-lambda"
  filename      = data.archive_file.lambda_zip.output_path
  source_code_hash = data.archive_file.lambda_zip.output_base64sha256
 
  role    = data.aws_iam_role.lab_role.arn 
  handler = "bootstrap" 
  runtime = "provided.al2" 
 
  memory_size = 512 
  timeout     = 10  

  # This resource depends on the zip file being ready
  depends_on = [data.archive_file.lambda_zip]
}

# 4. Give SNS permission to invoke this Lambda
resource "aws_lambda_permission" "sns_permission" {
  statement_id  = "AllowSNSInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.order_processor_lambda.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = module.messaging.sns_topic_arn 
}

# 5. Subscribe the Lambda to your existing SNS topic
resource "aws_sns_topic_subscription" "lambda_subscription" {
  topic_arn = module.messaging.sns_topic_arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.order_processor_lambda.arn
}

# 6. Create a Log Group for the Lambda
resource "aws_cloudwatch_log_group" "lambda_lg" {
  name              = "/aws/lambda/${aws_lambda_function.order_processor_lambda.function_name}"
  retention_in_days = 7
}
*/
