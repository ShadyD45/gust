---
title: Privacy and redaction
nav_order: 12
parent: Usage
---
# Privacy and redaction

{: .warning }
**Gust does not automatically redact sensitive data.** Anything your agent or exporter sends — prompts, tool arguments, tool outputs, headers, customer fields — can land in `AgentRun` JSON, fixtures, scenario proposals, HTML reports, datasets, and CI artifacts.

## What can leak

Typical paths:

- OTLP / Langfuse ingest → `run.json`
- Fixture recording → `fixtures/*.json`
- Mode 3 reports and GitHub step summaries
- `gust scenario from-run` / continuous-eval proposals
- Dataset bundles committed to git

Fields that often contain secrets or PII: emails, phone numbers, API tokens, auth headers, order payloads, internal prompts, database rows.

## What you should do today

1. **Prefer staging / synthetic data** for Mode 3 and ingest demos.
2. **Redact before commit or CI upload** — strip or hash env vars, Authorization headers, and known PII fields in your recorder or exporter.
3. **Use allowlists** for span attributes you actually need for assertions; drop the rest at capture time.
4. **Treat fixtures as sensitive** if they were recorded from real traffic.
5. **Review scenario proposals** before they enter a golden dataset (`reviewed_by` is mandatory for continuous-eval proposals).

Practical patterns:

| Pattern | Example |
|---------|---------|
| Header redaction | Drop or replace `Authorization`, `Cookie`, `X-API-Key` |
| Regex replacement | Emails, phone numbers, JWTs, card-like digit runs |
| JSON-path redaction | `$.customer.email`, `$.args.password` |
| Env scrubbing | Never dump process env into span attributes |

## `gust scenario propose`

Default PII redaction with audited opt-out:

```bash
# redaction ON by default
gust scenario propose runs/staging/ --output-dir proposals/ --limit 20

# audited opt-out (required note)
gust scenario propose run.json --no-redact --redact-opt-out-note "synthetic fixture pack"
```

Proposals never auto-fill assertions; `reviewed_by` stays empty until a human reviews the draft.

## Related

- [OTel ingestion]({% link usage/otel-ingest.md %})
- [Integrate your app]({% link usage/integrate-your-app.md %})
- [Invariants]({% link architecture/invariants.md %}) — a trace is never auto-asserted
