# 1. This is your DynamoDB "schema"
resource "aws_dynamodb_table" "shopping_carts" {
  name           = var.table_name
  billing_mode   = "PAY_PER_REQUEST"
  hash_key       = "PK"
  range_key      = "SK"

  # Primary Key attributes
  attribute {
    name = "PK"
    type = "S" # S = String
  }
  attribute {
    name = "SK"
    type = "S"
  }

  # Global Secondary Index (GSI) for finding carts by customer
  global_secondary_index {
    name            = "GSI1-CustomerIndex"
    hash_key        = "GSI1PK"
    range_key       = "GSI1SK"
    projection_type = "ALL"
  }

  # GSI key attributes
  attribute {
    name = "GSI1PK"
    type = "S"
  }
  attribute {
    name = "GSI1SK"
    type = "S"
  }

  tags = var.tags
}

#
# We have REMOVED the "aws_iam_role_policy" resource
# because the "LabRole" already has DynamoDB permissions
# and trying to modify it caused the "AccessDenied" error.
#
