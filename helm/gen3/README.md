# gen3

![Version: 0.3.60](https://img.shields.io/badge/Version-0.3.60-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: master](https://img.shields.io/badge/AppVersion-master-informational?style=flat-square)


Helm chart to deploy Gen3 Data Commons

## Local PostgreSQL and Syfon example

The default local profile deploys bundled PostgreSQL with
`global.postgres.tls.mode: selfSigned`. The chart creates and reuses the
`gen3-postgresql-tls` private-CA Secret, configures Bitnami PostgreSQL with
the server certificate, and gives Syfon plus its init Job only the CA with
`verify-full` trust. The generated SANs cover the release's PostgreSQL and
headless service names in short, namespace, `.svc`, and cluster-domain forms.

For an externally managed PostgreSQL server, set
`global.dev: false` and use `global.postgres.tls.mode: existingSecret` with
`global.postgres.tls.secretName` and `caKey`. `selfSigned` and `certManager`
are intentionally rejected for external PostgreSQL. `certManager` is
available for bundled PostgreSQL when a pre-existing Issuer or ClusterIssuer
is selected with `global.postgres.tls.certManager.issuerRef`; cert-manager is
not installed by this chart. A private/internal issuer must put the CA in the
resulting Secret under the configured `caKey`.

The Helm-generated self-signed CA and leaf are valid for 10 years. A Helm CLI
upgrade uses `lookup` to reuse the live Secret, so it does not rotate existing
material. Offline `helm template` and GitOps rendering cannot observe live
Secrets and will generate new material on each render; use `existingSecret` or
`certManager` for GitOps-managed release stability.

The TLS block is the source of truth for Syfon's `sslmode` and insecure
transport settings. With TLS enabled, both the Deployment and init Job use
`PGSSLMODE=verify-full` and the same CA mount. PostgreSQL must be restarted or
reloaded after a cert-manager leaf renewal; this chart has no built-in
certificate reloader. If the cluster already runs Stakater Reloader, opt in
through the Bitnami workload values, for example:

```yaml
postgresql:
  primary:
    podAnnotations:
      secret.reloader.stakater.com/reload: gen3-postgresql-tls
```

The local example sets `syfon.config.routes.docs: true`. Syfon's Swagger UI
and OpenAPI endpoints are public by design, including in production; other
API routes remain protected by the configured authentication and authorization.

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| ahilt | <ahilt@uchicago.edu> |  |
| ajoaugustine | <ajoa@uchicago.edu> |  |
| emalinowski | <emalinowski@uchicago.edu> |  |
| EliseCastle23 | <elisemcastle@uchicago.edu> |  |
| jawadqur | <qureshi@uchicago.edu> |  |

## Requirements

| Repository | Name | Version |
|------------|------|---------|
| file://../arborist | arborist | 0.1.35 |
| file://../aws-es-proxy | aws-es-proxy | 0.1.42 |
| file://../common | common | 0.1.37 |
| file://../frontend-framework | frontend-framework | 0.1.30 |
| file://../fence | fence | 0.1.79 |
| file://../funnel | funnel | 0.1.78 |
| file://../gecko | gecko | 0.1.0 |
| file://../requestor | requestor | 0.1.35 |
| file://../redis | redis | 0.1.0 |
| file://../revproxy | revproxy | 0.1.62 |
| https://charts.bitnami.com/bitnami | postgresql | 11.9.13 |
| file://../image-viewer | image-viewer | 0.1.0 |
| file://../viv | viv | 0.1.0 |
| file://../syfon | syfon | 0.1.2 |
| file://../loom | loom | 0.1.7 |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| arborist.enabled | bool | `true` | Whether to deploy the arborist subchart. |
| aws-es-proxy.enabled | bool | `false` | Whether to deploy the aws-es-proxy subchart. |
| aws-es-proxy.esEndpoint | str | `"test.us-east-1.es.amazonaws.com"` | Elasticsearch endpoint in AWS |
| aws-es-proxy.secrets | map | `{"awsAccessKeyId":"","awsSecretAccessKey":""}` | Secret information |
| aws-es-proxy.secrets.awsAccessKeyId | str | `""` | AWS access key ID for aws-es-proxy |
| aws-es-proxy.secrets.awsSecretAccessKey | str | `""` | AWS secret access key for aws-es-proxy |
| etl.enabled | bool | `true` | Whether to deploy the etl subchart. |
| fence.enabled | bool | `true` | Whether to deploy the fence subchart. |
| fence.usersync | map | `{"addDbgap":false,"onlyDbgap":false,"schedule":"*/30 * * * *","slack_send_dbgap":false,"slack_webhook":"None","syncFromDbgap":false,"userYamlS3Path":"s3://cdis-gen3-users/helm-test/user.yaml","usersync":false}` | Configuration options for usersync cronjob. |
| fence.usersync.addDbgap | bool | `false` | Force attempting a dbgap sync if "true", falls back on user.yaml |
| fence.usersync.onlyDbgap | bool | `false` | Forces ONLY a dbgap sync if "true", IGNORING user.yaml |
| fence.usersync.schedule | string | `"*/30 * * * *"` | The cron schedule expression to use in the usersync cronjob. Runs every 30 minutes by default. |
| fence.usersync.slack_send_dbgap | bool | `false` | Will echo what files we are seeing on dbgap ftp to Slack. |
| fence.usersync.slack_webhook | string | `"None"` | Slack webhook endpoint used with certain jobs. |
| fence.usersync.syncFromDbgap | bool | `false` | Whether to sync data from dbGaP. |
| fence.usersync.userYamlS3Path | string | `"s3://cdis-gen3-users/helm-test/user.yaml"` | Path to the user.yaml file in S3. |
| fence.usersync.usersync | bool | `false` | Whether to run Fence usersync or not. |
| frontend-framework | map | `{"enabled":true,"env":{"GEN3_INTERNAL_API":"http://revproxy-service"},"image":{"repository":"quay.io/cdis/frontend-framework","tag":"develop"}}` | Configurations for frontend-framework chart. |
| frontend-framework.enabled | bool | `true` | Whether to deploy the frontend-framework subchart. |
| frontend-framework.env | map | `{"GEN3_INTERNAL_API":"http://revproxy-service"}` | Environment variables for server-side frontend requests. |
| frontend-framework.env.GEN3_INTERNAL_API | string | `"http://revproxy-service"` | Internal reverse-proxy URL used for server-side requests. |
| frontend-framework.image | map | `{"repository":"quay.io/cdis/frontend-framework","tag":"develop"}` | Docker image information. |
| frontend-framework.image.repository | string | `"quay.io/cdis/frontend-framework"` | The Docker image repository for the frontend-framework. |
| frontend-framework.image.tag | string | `"develop"` | Overrides the image tag whose default is the chart appVersion. |
| global.aws | map | `{"awsAccessKeyId":null,"awsSecretAccessKey":null,"enabled":false,"useLocalSecret":{"enabled":false,"localSecretName":null}}` | AWS configuration |
| global.aws.awsAccessKeyId | string | `nil` | Credentials for AWS stuff. |
| global.aws.awsSecretAccessKey | string | `nil` | Credentials for AWS stuff. |
| global.aws.enabled | bool | `false` | Set to true if deploying to AWS. Controls ingress annotations. |
| global.aws.useLocalSecret | map | `{"enabled":false,"localSecretName":null}` | Local secret setting if using a pre-exising secret. |
| global.aws.useLocalSecret.enabled | bool | `false` | Set to true if you would like to use a secret that is already running on your cluster. |
| global.aws.useLocalSecret.localSecretName | string | `nil` | Name of the local secret. |
| global.ddEnabled | bool | `false` | Whether Datadog is enabled. |
| global.dev | bool | `true` | Deploys PostgreSQL for development |
| global.dictionaryUrl | string | `"https://s3.amazonaws.com/dictionary-artifacts/datadictionary/develop/schema.json"` | URL of the data dictionary. |
| global.dispatcherJobNum | int | `"10"` | Number of dispatcher jobs. |
| global.environment | string | `"default"` | Environment name. This should be the same as vpcname if you're doing an AWS deployment. Currently this is being used to share ALB's if you have multiple namespaces in same cluster. |
| global.externalSecrets | map | `{"dbCreate":false,"deploy":false}` | External Secrets settings. |
| global.externalSecrets.dbCreate | bool | `false` | Will create the databases and store the creds in Kubernetes Secrets even if externalSecrets is deployed. Useful if you want to use ExternalSecrets for other secrets besides db secrets. |
| global.externalSecrets.deploy | bool | `false` | Will use ExternalSecret resources to pull secrets from Secrets Manager instead of creating them locally. Be cautious as this will override secrets you have deployed. |
| global.hostname | string | `"localhost"` | Hostname for the deployment. |
| global.netPolicy | bool | `true` | Whether network policies are enabled. |
| global.postgres.dbCreate | bool | `true` | Whether the database create job should run. |
| global.postgres.master.host | string | `nil` | global postgres master host |
| global.postgres.master.password | string | `nil` | global postgres master password |
| global.postgres.master.port | string | `"5432"` | global postgres master port |
| global.postgres.master.username | string | `"postgres"` | global postgres master username |
| global.publicDataSets | bool | `true` | Whether public datasets are enabled. |
| global.revproxyArn | string | `"arn:aws:acm:us-east-1:123456:certificate"` | ARN of the reverse proxy certificate. |
| global.tierAccessLevel | string | `"libre"` | Access level for tiers. acceptable values for `tier_access_level` are: `libre`, `regular` and `private`. If omitted, by default common will be treated as `private` |
| global.tierAccessLimit | int | `"1000"` | Only relevant if tireAccessLevel is set to "regular". Summary charts below this limit will not appear for aggregated data. |
| manifestservice.enabled | bool | `true` | Whether to deploy the manifest service subchart. |
| peregrine.enabled | bool | `true` | Whether to deploy the peregrine subchart. |
| pidgin.enabled | bool | `true` | Whether to deploy the pidgin subchart. |
| postgresql | map | `{"primary":{"persistence":{"enabled":false}}}` | To configure postgresql subchart Disable persistence by default so we can spin up and down ephemeral environments |
| postgresql.primary.persistence.enabled | bool | `false` | Option to persist the dbs data. |
| requestor.enabled | bool | `false` | Whether to deploy the requestor subchart. |
| revproxy.enabled | bool | `true` | Whether to deploy the revproxy subchart. |
| revproxy.ingress.annotations | map | `{}` | Annotations to add to the ingress. |
| revproxy.ingress.enabled | bool | `false` | Whether to create the custom revproxy ingress |
| revproxy.ingress.hosts | list | `[{"host":"chart-example.local"}]` | Where to route the traffic. |
| revproxy.ingress.tls | list | `[]` | To secure an Ingress by specifying a secret that contains a TLS private key and certificate. |
| sheepdog.enabled | bool | `true` | Whether to deploy the sheepdog subchart. |
