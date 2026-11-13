---
title: "Self-Hosting Guide"
description: "Run Vigil on your own infrastructure."
---

# Self-Hosting Guide

## Prerequisites

- Kubernetes 1.28+ cluster (EKS, GKE, or local kind/k3s)
- Helm 3.12+
- Postgres 16, Redis 7, ClickHouse 24+ (or use the provided Terraform modules)
- AWS Secrets Manager (or substitute with Vault / Kubernetes Secrets)

## 1. Add the Vigil Helm repository

```bash
helm repo add vigil https://charts.vigil.dev
helm repo update
```

## 2. Create a namespace and External Secret store

```bash
kubectl create namespace vigil

# If using AWS Secrets Manager + External Secrets Operator:
kubectl apply -f deploy/k8s/vigil/templates/externalsecret.yaml
```

Create a secret in AWS Secrets Manager at path `vigil/<env>` with keys:
```
DATABASE_URL=postgres://vigil:password@db:5432/vigil?sslmode=require
REDIS_URL=redis://:password@redis:6379/0
CLICKHOUSE_DSN=clickhouse://vigil:password@clickhouse:9000/vigil
JWT_SECRET=<32+ bytes random>
ANTHROPIC_API_KEY=sk-ant-...
ALERT_SIGNING_KEY=<32+ bytes random>
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/...
```

## 3. Install the chart

```bash
helm install vigil ./deploy/k8s/vigil \
  --namespace vigil \
  --set global.image.tag=latest \
  --set global.externalSecrets.secretStoreName=vigil-aws \
  --set global.externalSecrets.secretName=vigil/prod
```

## 4. Run database migrations

```bash
# Postgres
kubectl -n vigil exec deploy/vigil-api -- \
  /app/vigil-api migrate up

# ClickHouse
kubectl -n vigil exec deploy/vigil-ingest -- \
  /app/vigil-ingest migrate up
```

## 5. Verify

```bash
kubectl -n vigil get pods
# All pods should be Running/Ready

curl https://your-vigil-domain/healthz
# {"status":"ok"}
```

## Terraform (AWS)

```bash
cd deploy/terraform/staging
terraform init
terraform workspace select staging
terraform plan -var-file=staging.tfvars
terraform apply
```

For production:
```bash
cd deploy/terraform/prod
terraform workspace select prod
terraform plan -var-file=prod.tfvars
# Review carefully, then:
terraform apply
```

## Configuration reference

All configuration is via environment variables. See `.env.example` at the repo root for the full list with descriptions.

## Backups

- **Postgres**: RDS automated backups with PITR are enabled by default in the Terraform modules.
- **ClickHouse**: Configure `deploy/clickhouse/backup.xml` to point at your S3 bucket; the `s3` module provisions the bucket.
- **Redis**: ElastiCache daily snapshots enabled; for point-in-time recovery, enable AOF logging.

## Upgrading

```bash
helm upgrade vigil ./deploy/k8s/vigil \
  --namespace vigil \
  --set global.image.tag=<new-tag> \
  --atomic  # auto-rollback on failure
```
