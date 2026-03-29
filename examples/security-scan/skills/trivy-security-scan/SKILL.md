---
name: trivy-security-scan
description: >
  Comprehensive security scanner for filesystems, container images, and IaC.
  Detects known CVEs in dependencies, hardcoded secrets, and IaC misconfigurations
  in Terraform, Kubernetes, Dockerfiles, and CloudFormation.
applyTo: "**"
---

# Trivy Security Scan Skill

Trivy by Aqua Security is a multi-target, multi-scanner tool that detects CVEs, secrets, misconfigurations, and license issues across containers, filesystems, and cloud-native infrastructure.

## Core Commands

```bash
# Filesystem vulnerability + secret scan (most common)
trivy fs --scanners vuln,secret ./

# Filter by severity
trivy fs --scanners vuln,secret --severity HIGH,CRITICAL ./

# JSON output for automation
trivy fs --scanners vuln,secret --format json -o results.json ./

# IaC misconfiguration scan
trivy config ./

# Scan specific IaC directory
trivy config --severity HIGH,CRITICAL ./terraform/

# Full scan: vulns + secrets + misconfigs
trivy fs --scanners vuln,secret,misconfig ./

# Skip noisy directories
trivy fs --skip-dirs node_modules,.git,vendor --scanners vuln,secret ./

# Ignore unfixed CVEs
trivy fs --ignore-unfixed --scanners vuln ./
```

## Scanner Types

### Vulnerability Scanner (`vuln`)
Detects known CVEs in:
- **OS packages**: Alpine, Debian, Ubuntu, RHEL, Amazon Linux
- **Language packages**: npm, pip, gem, cargo, go modules, NuGet, Maven
- **Manifests**: package-lock.json, requirements.txt, go.sum, Cargo.lock

### Secret Scanner (`secret`)
Detects:
- AWS access keys, GCP service accounts, Azure credentials
- GitHub/GitLab/Slack tokens, Stripe API keys
- SSH private keys, TLS certificates, PGP keys
- Passwords, database connection strings, JWTs

### Misconfiguration Scanner (`misconfig`)
Checks:
- **Terraform**: Public S3, unencrypted storage, overly permissive IAM
- **Kubernetes**: Privileged containers, missing limits, exposed secrets
- **Dockerfile**: Running as root, no HEALTHCHECK, insecure base images
- **CloudFormation**: Unencrypted resources, public access
- **GitHub Actions**: Unpinned actions, shell injection in workflows

## MITRE ATT&CK Mappings

| Finding Type | MITRE ATT&CK |
|-------------|---------------|
| CVEs in dependencies | T1195.001 (Supply Chain: Dependencies) |
| CVEs in container images | T1195.002 (Supply Chain: Software) |
| Hardcoded credentials | T1552.001 (Credentials in Files) |
| Private keys | T1552.004 (Private Keys) |
| Dockerfile misconfigs | T1610 (Deploy Container) |
| K8s misconfigs | T1613 (Container Discovery) |

## Severity Levels

| Severity | CVSS Score | Action |
|----------|------------|--------|
| CRITICAL | 9.0 - 10.0 | Immediate fix required |
| HIGH | 7.0 - 8.9 | Fix before deployment |
| MEDIUM | 4.0 - 6.9 | Plan remediation |
| LOW | 0.1 - 3.9 | Document and monitor |

## Limitations

- No source code vulnerability scanning — use `bandit` or `graudit` for that
- No malware detection — use `guarddog` for malicious packages
- Database must be updated for latest CVEs (auto-downloads on first run)
- Secret detection is pattern-based — may flag non-sensitive strings
- First run downloads ~200MB vulnerability database
