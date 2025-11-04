# --- 1. SNS Topic: order-processing-events ---
resource "aws_sns_topic" "order_events" {
  name = "order-processing-events"
}

# --- 2. SQS Queue: order-processing-queue ---
resource "aws_sqs_queue" "order_queue" {
  name                       = "order-processing-queue"
  visibility_timeout_seconds = 30     # Requirement: 30 seconds
  message_retention_seconds  = 345600 # Requirement: 4 days (345600 seconds)
  receive_wait_time_seconds  = 20     # Requirement: 20 seconds (long polling)
}

# --- 3. SQS Queue Policy (Allows SNS to Send Messages) ---
data "aws_iam_policy_document" "sqs_policy_doc" {
  statement {
    effect    = "Allow"
    principals {
      type        = "Service"
      identifiers = ["sns.amazonaws.com"]
    }
    actions   = ["sqs:SendMessage"]
    resources = [aws_sqs_queue.order_queue.arn]
    condition {
      test     = "ArnEquals"
      variable = "aws:SourceArn"
      values   = [aws_sns_topic.order_events.arn]
    }
  }
}

resource "aws_sqs_queue_policy" "order_queue_policy" {
  queue_url = aws_sqs_queue.order_queue.id
  policy    = data.aws_iam_policy_document.sqs_policy_doc.json
}

# --- 4. SNS Subscription (Connects Topic to Queue) ---
resource "aws_sns_topic_subscription" "queue_subscription" {
  topic_arn              = aws_sns_topic.order_events.arn
  protocol               = "sqs"
  endpoint               = aws_sqs_queue.order_queue.arn
  raw_message_delivery   = true # Recommended for simpler JSON parsing
}