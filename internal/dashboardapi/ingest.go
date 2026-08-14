// ingest.go: POST /control/ingest - nap DU LIEU MOI vao he thong de fleet hoc.
//
// Khac /control/feed: feed chi tra cac case cu ve hang doi. Ingest sinh giao dich
// MOI co nhan (fraud/legit) va tao task cho chung - tuc la thuc su lam giau kho
// case, va qua do lam giau episodic memory. Day la duong nap lieu cua Training Lab.
//
// Trung thuc: giao dich sinh o day duoc gan risk_score tong hop nam trong dai
// dieu tra, khong di qua XGBoost scorer. Muc dich la nuoi tang suy luan + bo nho,
// khong phai danh gia tang scoring - dieu do duoc ghi ro tren giao dien.
package dashboardapi

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

type ingestResult struct {
	Status    string `json:"status"`
	Inserted  int    `json:"inserted"`
	Fraud     int    `json:"fraud"`
	Legit     int    `json:"legit"`
	Seed      int64  `json:"seed"`
	TookMs    int64  `json:"took_ms"`
	QueuedNow int    `json:"queued_now"`
}

const ingestSQL = `
WITH t AS (
	INSERT INTO transactions (
		step, type, amount, name_orig, old_balance_orig, new_balance_orig,
		name_dest, old_balance_dest, new_balance_dest,
		error_balance_orig, error_balance_dest,
		risk_score, risk_tier, is_fraud_label
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'medium',$13)
	RETURNING id, risk_score
)
INSERT INTO tasks (transaction_id, risk_score, status)
SELECT id, risk_score, 'pending' FROM t`

func (s *Server) IngestData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !controlAllowed(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var body struct {
		Count     int     `json:"count"`
		FraudRate float64 `json:"fraud_rate"`
		Seed      int64   `json:"seed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if body.Count < 1 || body.Count > 300 {
		http.Error(w, "count must be 1-300 (larger sets belong in scripts/init.sh)", http.StatusBadRequest)
		return
	}
	if body.FraudRate < 0 || body.FraudRate > 1 {
		http.Error(w, "fraud_rate must be 0-1", http.StatusBadRequest)
		return
	}
	if body.Seed == 0 {
		body.Seed = time.Now().UnixNano()
	}

	start := time.Now()
	rng := rand.New(rand.NewSource(body.Seed))
	ctx := r.Context()
	res := ingestResult{Status: "ok", Seed: body.Seed}

	for i := 0; i < body.Count; i++ {
		txn := synthesise(rng, rng.Float64() < body.FraudRate)
		if _, err := s.DB.Pool.Exec(ctx, ingestSQL,
			txn.step, txn.txType, txn.amount, txn.nameOrig, txn.oldOrig, txn.newOrig,
			txn.nameDest, txn.oldDest, txn.newDest, txn.errOrig, txn.errDest,
			txn.risk, txn.isFraud,
		); err != nil {
			http.Error(w, fmt.Sprintf("ingest failed after %d rows: %v", res.Inserted, err), http.StatusInternalServerError)
			return
		}
		res.Inserted++
		if txn.isFraud {
			res.Fraud++
		} else {
			res.Legit++
		}
	}

	_ = s.DB.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM tasks WHERE status = 'pending'`).Scan(&res.QueuedNow)
	res.TookMs = time.Since(start).Milliseconds()
	writeJSON(w, res)
}

type synthTxn struct {
	step             int
	txType           string
	amount           float64
	nameOrig         string
	oldOrig, newOrig float64
	nameDest         string
	oldDest, newDest float64
	errOrig, errDest float64
	risk             float64
	isFraud          bool
}

// synthesise dung dung chu ky ma cong cu doi soat so du nhan ra:
//   - fraud: tai khoan goc giu xap xi so tien roi bi vet ve 0, dich khong nhan
//   - legit: goc con du sau khi tru, dich duoc ghi co dung so tien
//
// risk_score luon nam trong dai dieu tra de case di toi agent (day la du lieu
// huan luyen, khong phai bai kiem tra cua scorer).
func synthesise(rng *rand.Rand, fraud bool) synthTxn {
	t := synthTxn{
		step:     1 + rng.Intn(743),
		txType:   "TRANSFER",
		nameOrig: fmt.Sprintf("C%09d", rng.Intn(999999999)),
		nameDest: fmt.Sprintf("C%09d", rng.Intn(999999999)),
		isFraud:  fraud,
		risk:     0.05 + rng.Float64()*0.9, // trong dai (low, high)
	}
	if rng.Intn(2) == 0 {
		t.txType = "CASH_OUT"
	}

	if fraud {
		t.amount = round2(1000 + rng.Float64()*400000)
		t.oldOrig, t.newOrig = t.amount, 0
		t.oldDest, t.newDest = 0, 0
	} else {
		t.amount = round2(10 + rng.Float64()*80000)
		t.oldOrig = round2(t.amount + rng.Float64()*200000)
		t.newOrig = round2(t.oldOrig - t.amount)
		t.oldDest = round2(rng.Float64() * 100000)
		t.newDest = round2(t.oldDest + t.amount)
	}

	t.errOrig = round2(t.newOrig + t.amount - t.oldOrig)
	t.errDest = round2(t.oldDest + t.amount - t.newDest)
	return t
}

func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }
