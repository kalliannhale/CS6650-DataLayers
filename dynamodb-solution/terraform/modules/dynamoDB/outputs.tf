output "table_name" {
  description = "The name of the DynamoDB shopping cart table."
  value       = aws_dynamodb_table.shopping_carts.name
}

output "table_arn" {
  description = "The ARN of the DynamoDB shopping cart table."
  value       = aws_dynamodb_table.shopping_carts.arn
}
