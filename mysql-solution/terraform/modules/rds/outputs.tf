output "db_instance_address" {
  description = "The hostname of the RDS instance"
  value       = aws_db_instance.mysql.address
}

output "db_instance_port" {
  description = "The database port"
  value       = aws_db_instance.mysql.port
}

output "db_username" {
  description = "The databse username"
  value       = aws_db_instance.mysql.username
  sensitive   = true
}

output "db_password" {
  value     = random_password.db_password.result
  sensitive = true
}

output "db_name" {
  value     = aws_db_instance.mysql.db_name
}
