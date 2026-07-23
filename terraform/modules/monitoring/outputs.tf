output "sns_alerts_arn" { value = aws_sns_topic.alerts.arn }
output "dashboard_name" { value = aws_cloudwatch_dashboard.hivemind.dashboard_name }
output "dashboard_url" {
  value = "https://console.aws.amazon.com/cloudwatch/home#dashboards:name=${aws_cloudwatch_dashboard.hivemind.dashboard_name}"
}
