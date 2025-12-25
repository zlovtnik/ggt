# Data Protection Runbook

This document outlines operational procedures and responsibilities for protecting personal and sensitive data processed by the Transform Service.

Owners
- Data Protection / Privacy Team: data-protection-team@example.com
- Service Owner: platform-ops@example.com

Compliance
- Applicable regulations: GDPR, CCPA (verify regional applicability before processing personal data).

Encryption
- Encryption in transit: all service-to-service communication must use TLS 1.2+ (prefer TLS 1.3).
- Encryption at rest: any persistent storage (databases, object stores) that stores PII or secrets must use disk-level or application-level encryption. Keys must be managed via a KMS (HashiCorp Vault, AWS KMS, GCP KMS, or equivalent).

Credentials and Secrets
- Store secrets in a centralized secret manager. Rotate keys regularly and follow least-privilege access.

Retention & Erasure
- Default retention: `default_retention_days` is declared in the schema metadata; concrete retention policies must be defined per pipeline and enforced by data lifecycle jobs.
- Data subject requests (DSR): follow the incident runbook for erasure requests—locate all copies (hot caches, backups, logs) and coordinate with Storage/DB owners.

Access Controls & Logging
- Apply RBAC to production systems, and maintain an access review cadence (quarterly).
- Audit/log accesses to systems containing PII; retain audit logs according to company policy and compliance requirements.

Minimization & Validation
- Only collect fields necessary for the declared processing purpose. Use schema `maxLength`, `pattern` and `required` constraints to limit data.

Operational Procedures
- Key management: KMS-backed keys, restricted IAM roles, and documented rotation schedule.
- Backups: ensure backups are encrypted and have documented retention/erasure processes.
- Incident escalation: contact data-protection-team@example.com and follow the incident response playbook.

References
- See `configs/config.example.yaml` for sample config fields (`operation_timeout`, `retry_count`, etc.).
- See `docs/architecture.md` for high-level architecture notes.
