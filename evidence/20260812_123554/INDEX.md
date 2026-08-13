# Evidence capture

Snapshot tu he thong dang chay (dashboard-api -> CockroachDB / CloudWatch / GitHub la nguon that, khong dan dung tay).

| File | Chung minh |
|------|-----------|
| overview.json | Verdict hom nay, accuracy vs ground-truth, learning curve |
| memory.json | Episodic memory: active/archived, salience, patterns, impact |
| verdict-vs-groundtruth.json | Ma tran verdict x is_fraud_label (recall/precision) |
| memory-top-recalled.json | Case duoc recall/merge nhieu nhat (fleet dang hoc) |
| crash-recovery-events.json | task_requeued -> task_resumed (chaos test) |
| fleet-distinct-agents.json | So agent phan biet da claim task (concurrency) |
| regions.json | Cau hinh multi-region + survival goal |
| audit-actions.json, task-status.json, db-stats.json, fleet.json, cost.json, lambdas.json, infrastructure.json | Trang thai van hanh tong the |
