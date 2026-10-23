# Goals Checkpoint Schema

Moved from SKILL.md "State and criteria". Read when creating or updating the goals checkpoint.

The checkpoint is a JSON object with a top-level `goals` array and sibling
`metrics` object, not a bare goal or array. Initialize all five metrics to 0.0:

```json
{
  "goals": [
    {
      "id": "G1",
      "title": "Short goal title",
      "objective": "Concrete deliverable description",
      "status": "pending",
      "successCriteria": [
        {
          "id": "G1-C1",
          "scenario": "curl -i http://localhost:3000/api/x | expect 200 + body.id",
          "channel": "HTTP call",
          "expectedEvidence": ".issueops/verified-execution/evidence/G1-C1.txt",
          "status": "pending",
          "capturedEvidence": null,
          "cleanupReceipt": null,
          "ultraqaClasses": ["malformed_input", "stale_state"]
        }
      ]
    }
  ],
  "metrics": {
    "evidenceCoverage": 0.0,
    "reworkRate": 0.0,
    "cycleEfficiency": 0.0,
    "parallelizationRatio": 0.0,
    "cleanupCompliance": 0.0
  }
}
```
