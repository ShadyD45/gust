# Mock-provider calibration dry-run

This report is produced by:

```bash
./gust judge calibrate --provider mock --dataset testdata/judge/calibration/v1.json --out docs/usage/judge-calibration-mock.json
```

The checked-in [`judge-calibration-mock.json`](judge-calibration-mock.json) shows a passing Spearman ρ on the synthetic seed set (≥50 cases). The mock provider is **not** a production calibration — replace human labels and re-run against your official SDK plugin before setting `llm_judge.calibrated: true` in policy.
