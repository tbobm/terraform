module github.com/hashicorp/terraform/internal/backend/remote-state/s3

go 1.23.1

require (
	github.com/aws/aws-sdk-go-v2 v1.30.5
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.13
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.17.18
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.34.9
	github.com/aws/aws-sdk-go-v2/service/s3 v1.61.2
	github.com/aws/smithy-go v1.20.4
	github.com/google/go-cmp v0.6.0
	github.com/hashicorp/aws-sdk-go-base/v2 v2.0.0-beta.56
	github.com/hashicorp/go-hclog v1.6.3
	github.com/hashicorp/go-uuid v1.0.3
	github.com/hashicorp/hcl/v2 v2.22.0
	github.com/hashicorp/terraform v0.0.0-00010101000000-000000000000
	github.com/zclconf/go-cty v1.15.0
)

require (
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/apparentlymart/go-versions v1.0.2 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.4 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.27.33 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.32 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.3.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/iam v1.35.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.11.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.3.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.9.18 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.11.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.17.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/sqs v1.34.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.22.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.26.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.30.7 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fatih/color v1.17.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-slug v0.15.0 // indirect
	github.com/hashicorp/go-version v1.6.0 // indirect
	github.com/hashicorp/terraform-plugin-log v0.9.0 // indirect
	github.com/hashicorp/terraform-registry-address v0.2.3 // indirect
	github.com/hashicorp/terraform-svchost v0.1.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/spf13/afero v1.9.5 // indirect
	go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws v0.55.0 // indirect
	go.opentelemetry.io/otel v1.30.0 // indirect
	go.opentelemetry.io/otel/metric v1.30.0 // indirect
	go.opentelemetry.io/otel/trace v1.30.0 // indirect
	golang.org/x/mod v0.21.0 // indirect
	golang.org/x/net v0.29.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.25.0 // indirect
	golang.org/x/text v0.18.0 // indirect
	golang.org/x/tools v0.25.0 // indirect
)

replace github.com/hashicorp/terraform/internal/backend/remote-state/azure => ../azure

replace github.com/hashicorp/terraform/internal/backend/remote-state/consul => ../consul

replace github.com/hashicorp/terraform/internal/backend/remote-state/cos => ../cos

replace github.com/hashicorp/terraform/internal/backend/remote-state/gcs => ../gcs

replace github.com/hashicorp/terraform/internal/backend/remote-state/kubernetes => ../kubernetes

replace github.com/hashicorp/terraform/internal/backend/remote-state/oss => ../oss

replace github.com/hashicorp/terraform/internal/backend/remote-state/pg => ../pg

replace github.com/hashicorp/terraform/internal/backend/remote-state/s3 => ../s3

replace github.com/hashicorp/terraform/internal/legacy => ../../../legacy

replace github.com/hashicorp/terraform/internal => ../../..

replace github.com/hashicorp/terraform => ../../../..
