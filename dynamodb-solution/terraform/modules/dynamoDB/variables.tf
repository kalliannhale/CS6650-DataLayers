variable "table_name" {
  description = "The name of the DynamoDB table for shopping carts."
  type        = string
  default     = "shopping-carts-table"
}

variable "tags" {
  description = "A map of tags to apply to the resources."
  type        = map(string)
  default     = {}
}
