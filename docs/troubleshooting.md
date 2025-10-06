## Troubleshooting

### Instance not found

- Ensure the instance name matches exactly (case-sensitive)
- Run `ssm sync` to refresh the instance cache
- Check that SSM agent is installed and running on the instance

### Permission denied

- Verify your AWS credentials are configured correctly
- Ensure your IAM user/role has the required permissions
- Check that the instance exists and is in a running state

### Connection issues

- Ensure AWS CLI is installed and configured
- Check network connectivity to AWS
- Verify the instance is reachable via SSM (check ping status)


