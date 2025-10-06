## Installation

### Build from source

```bash
git clone https://github.com/andreclaro/ssm
cd ssm
go build -o ssm .
```

### Download pre-built binary

```bash
# Download the binary for your platform
curl -L https://github.com/andreclaro/ssm/releases/download/v1.0.0/ssm-$(uname -s)-$(uname -m) -o ssm
chmod +x ssm
sudo mv ssm /usr/local/bin/
```

### Prerequisites

- AWS CLI installed and configured
- SSM Agent running on EC2 instances
- Appropriate IAM permissions

#### Required IAM permissions (example)

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeInstances",
        "ssm:DescribeInstanceInformation",
        "ssm:StartSession",
        "sts:GetCallerIdentity"
      ],
      "Resource": "*"
    }
  ]
}
```


