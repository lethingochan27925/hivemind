# HiveMind — Kịch bản quay CHÍNH THỨC (v6 · mục tiêu 2:55 / hạn 3:00)

Hợp nhất bản v4 (draft trước, mạnh về kỹ thuật chứng minh trực tiếp trên console)
với tình trạng dự án hôm nay. **Cập nhật `16/08 23:42 UTC`: đã build lại dữ liệu +
xác minh xong qua `make scorecard`** — 118 memories / 4.321 case, 4.988 lần resume,
multi-region 3 vùng đã bật (Singapore primary + Mumbai + Jakarta). Số liệu trong
2 cảnh dưới đây (cảnh 2, cảnh 4) đã là số THẬT, sẵn sàng quay — không cần build lại
nữa trừ khi bạn quay cách xa mốc thời gian trên nhiều ngày.

Sáu cảnh, thoại tiếng Anh (giám khảo quốc tế), chỉ dẫn quay tiếng Việt.
Mỗi cảnh cố ý nhắm vào đúng 1 (hoặc 2) tiêu chí chấm điểm — ghi rõ dưới mỗi cảnh,
để nếu phải cắt bớt thì biết cắt cảnh nào là mất đúng tiêu chí nào.

**Nguyên tắc giữ nguyên từ v4, quan trọng nhất: đừng nói miệng con số nào mà bạn
không vừa nhìn thấy trên màn hình trong 5 phút gần nhất.** Camera/SQL console tự
chứng minh; thoại chỉ diễn giải, không tự bịa số.

---

## ⚠️ TRƯỚC TIÊN — tình trạng hệ thống (cập nhật `16/08 23:42 UTC`)

- Database: **đã có dữ liệu thật** — 118 memories / 4.321 case hấp thụ, xác minh qua `make scorecard`.
- Schema: **đã áp dụng** — `run_schema.py` chạy xong (bao gồm cả 3 migration, có fix bug hôm nay).
- Multi-region: **đã bật, 3 vùng** — Singapore (`ap-southeast-1`, primary) + Mumbai (`ap-south-1`) +
  Jakarta (`ap-southeast-3`, thay Sydney vì không có sẵn cho gói Basic lúc này), `survival_goal: "region"`
  đã xác nhận qua `make regions`. Cảnh 5 nhịp 3 quay được bình thường.
- Chaos/resume: **đã có lịch sử thật** — 8.856 re-queue, 4.988 resume, xác minh qua `make scorecard`.
- Dashboard URL: **đổi khác mỗi lần destroy/rebuild** — lấy URL mới nhất từ `SUBMISSION.md` hoặc
  `terraform output -raw dashboard_url`, đừng dùng URL cũ trong đầu nếu bạn destroy/rebuild lại lần nữa.
- CI/CD pipeline: `bash test/integration/pipeline_test.sh` hiện báo 117 passed (số này tăng theo thời
  gian khi thêm coverage mới — đừng hardcode, đọc lại lúc quay).
- **Nếu bạn destroy/rebuild lại hạ tầng sau mốc thời gian trên**: coi như mọi con số ở đây (memory,
  resume, multi-region) mất hiệu lực, làm lại từ Bước 3 trong phần CHUẨN BỊ bên dưới.

---

## CHUẨN BỊ — làm đúng thứ tự (không multi-region: ~20 phút · có multi-region: ~50 phút)

**✅ Bước 1, 3, 4, 5 đã làm xong tính tới `16/08 23:42 UTC`** (schema áp dụng, ~500+ case đã nạp,
chaos test đã chạy, multi-region 3 vùng đã bật) — giữ lại nguyên văn bên dưới **chỉ để tham khảo nếu
bạn destroy/rebuild lại hạ tầng lần nữa** trước khi quay. Nếu chưa destroy lại, bỏ qua thẳng xuống
Bước 2 (email) → Bước 6 (evidence, nên làm lại ngay sát giờ quay) → Bước 7 → Bước 8.

### Bước 1 — Xác nhận schema đã có (2 phút, đừng bỏ qua)
```bash
cd /mnt/d/code/hivemind
set -a; . ./.env; set +a
python3 -c "import psycopg2" 2>/dev/null && echo "psycopg2 OK" || pip install psycopg2-binary
python3 scripts/run_schema.py
```
An toàn để chạy lại dù đã chạy — mọi migration đều `IF NOT EXISTS`/idempotent.

### Bước 2 — Xác nhận 2 email cảnh báo billing (1 phút)
Kiểm tra hộp thư có 2 email xác nhận subscription từ AWS Notifications (topic `hivemind-dev-alerts`
và `hivemind-dev-billing-alerts`, tách riêng vì billing alarm ở us-east-1). Không ảnh hưởng video,
làm 1 lần cho xong.

### Bước 3 — Build dữ liệu chứng minh từ SỐ 0 (10-15 phút, quan trọng nhất)
Hệ thống hiện KHÔNG có gì để quay — phải nạp đủ để cảnh 2 (memory) và cảnh 3 (SQL 100%/100%) có
số đẹp:
```bash
make start
make feed N=100 && make dispatch
# đợi hàng đợi rút cạn (Mission Control: "0 pending · 0 active"), lặp lại:
make feed N=100 && make dispatch
make feed N=100 && make dispatch
```
Nạp tối thiểu ~250-300 case, càng nhiều càng có nhiều memory consolidation đẹp để quay cảnh 2.
Kiểm tra `/memory` (Fleet & Memory) thấy **Active cases** > 20-30 thì đủ dùng — đừng cố chờ tới
con số như bản draft cũ (115/3.873), thời gian không cho phép build lại từ đầu tới mức đó.

### Bước 4 — Build lịch sử "chết & phục hồi" thật cho cảnh 4 (5 phút)
DB mới nên **0 lần resume** — bản v4 định nói "over three thousand times", giờ không còn đúng.
Chạy vài lần TRƯỚC khi quay để có ít nhất vài lần thật trong audit log (không bắt buộc nói số cụ thể,
xem phần "sửa thoại cảnh 4" bên dưới):
```bash
make feed N=20 && make dispatch
# vào /infrastructure, bấm "Simulate agent crash" 2-3 lần, mỗi lần đợi tracker chạy xong (~30-60s)
```

### Bước 5 — (Tuỳ chọn, cân nhắc kỹ thời gian còn lại tới hạn nộp) Multi-region thật
```bash
./scripts/multi-region.sh enable aws-ap-southeast-3 aws-ap-south-1
```
(Jakarta thay Sydney — `ap-southeast-2` không có sẵn cho gói Basic lúc setup, `ap-southeast-3` đã
xác nhận hoạt động.)
Script dừng giữa chừng, cần bạn vào **CockroachDB Cloud console → cluster → Regions → Add region**
cho cả 2 region, đợi "ready" rồi Enter tiếp. Cổng chặn bắt buộc:
```bash
make regions
```
Chỉ quay cảnh 5 nhịp 3 (multi-region) khi thấy **3 hàng region** và `"survival_goal": "region"`.
**Nếu thời gian gấp, bỏ qua bước này** — cảnh 5 vẫn quay được đầy đủ chỉ với nhịp 1+2 (human-in-the-loop),
bỏ nhịp 3 và câu "spans three AWS regions" khỏi thoại. Đây KHÔNG phải tiêu chí bắt buộc để có video tốt.

### Bước 6 — Số liệu tươi + evidence khớp ngày quay
```bash
make scorecard
make evidence L=recording-day
bash test/integration/pipeline_test.sh   # đọc dòng cuối "N passed, 0 failed" — dùng cho B-roll cảnh 6, KHÔNG nói miệng
go run ./cmd/eval --api "$(terraform -chdir=terraform output -raw dashboard_api_url)"
```

### Bước 7 — Đảm bảo pipeline xanh cho cảnh 6
Vào GitHub Actions, nếu lần chạy gần nhất không phải hôm nay, push 1 commit nhỏ hoặc re-run để có
chuỗi Build → Staging → Smoke → Canary xanh và mới trên màn hình lúc quay.

### Bước 8 — Trình duyệt
Cửa sổ sạch, ẩn bookmark bar, zoom 100%, chọn **EN**, dark theme.

**Mở sẵn 6 tab theo thứ tự cảnh:**
1. `/` (Mission Control)
2. `/transactions`
3. `/database`
4. `/infrastructure` — cuộn sẵn tới panel **Chaos test** (và **Multi-region** nếu làm Bước 5)
5. `/reviews`
6. GitHub Actions — trang run list (toàn ✓, mới nhất là hôm nay)

**Câu SQL A** (cảnh 3 — bằng chứng 100%/100% sống, không phải slide):
```sql
SELECT CASE WHEN (ABS(tx.old_balance_orig - tx.amount) < 1 AND tx.new_balance_orig < 1)
         OR (tx.new_balance_dest >= tx.old_balance_dest + tx.amount - 1)
       THEN 'traceable' ELSE 'untraceable' END AS money_flow,
       t.verdict, COUNT(*)
FROM tasks t JOIN transactions tx ON tx.id = t.transaction_id
WHERE t.verdict IS NOT NULL GROUP BY 1,2 ORDER BY 1,2
```

**Câu SQL B** (cảnh 5 — bằng chứng ký ức người dạy, chạy trên `/database`):
```sql
SELECT left(summary, 80) AS lesson, salience
FROM case_memory WHERE 'human_reviewed' = ANY(key_signals)
```

**Chạy thử một lượt không thu tiếng để canh timing.** Thu 1080p (OBS), mic riêng nếu có.

---

## CẢNH 1 — Mission Control (0:00 – 0:28)
*Nhắm: Technical Implementation*

**Hình:** tab `/`. KPI có số, Live fleet có agent chạy. Rê chuột qua hàng KPI, dừng ở banner trạng
thái, bấm **"Feed 100 more"**.

**Thoại:**
> "This is HiveMind — a fleet of fraud-investigation agents on AWS Lambda,
> with one shared brain in CockroachDB.
> One click: a hundred cases go in. Agents claim work with
> SELECT FOR UPDATE SKIP LOCKED — no two agents ever grab the same case.
> Zero double-claims, ever."

---

## CẢNH 2 — Decision trace (0:28 – 1:00)
*Nhắm: Agentic Memory Design*

**Hình:** tab `/transactions`, bấm một dòng verdict `fraud`. Cuộn CHẬM qua Decision trace: memory
recall (similarity), Bedrock reasoning, verdict + confidence.

**Thoại:**
> "Every verdict has an audit trail. This agent recalled similar past fraud
> from vector memory — real cosine similarities, from a hundred eighteen
> consolidated memories distilled from over four thousand past cases.
> It reasoned with Claude on Bedrock, and decided — for about two hundredths
> of a cent per investigation."

*(✅ ĐÃ XÁC MINH `16/08 23:42 UTC` qua `make scorecard`: 118 memories hấp thụ 4.321 case (~36.6:1),
trung bình 2.91 memory được gợi nhớ mỗi lần điều tra, $0.00023/case. Số thật, đủ lớn để nói thẳng, không
cần né nữa. Nếu quay CÁCH xa thời điểm này (>1-2 ngày), chạy lại `make scorecard` lần nữa trước khi
quay để chắc số chưa trôi quá xa — nhưng không cần lo bị nhỏ đi, số này chỉ tăng theo thời gian.)*

---

## CẢNH 3 — Bằng chứng SQL sống (1:00 – 1:30)
*Nhắm: Real-World Impact + Creativity (chứng minh, không tuyên bố)*

**Hình:** tab `/database`. Dán **câu A**, bấm Run, chỉ vào bảng kết quả.

**Thoại:**
> "Don't take my word for it — this console runs read-only SQL on the live
> database, and it's public.
> When the money flow is traceable, the agent decides: one hundred percent
> of cases, one hundred percent accurate against ground truth.
> When the money vanishes without a trace, it escalates to a human.
> Every single time. It decides everything it can prove — and knows
> exactly what it can't."

*(Câu cuối là câu quan trọng nhất trong cả video — nó biến việc "agent không tự tin đoán bừa" thành
điểm mạnh chủ động thay vì điểm yếu bị hỏi. Giữ nguyên, đừng cắt.)*

---

## CẢNH 4 — Chaos: giết agent giữa điều tra (1:30 – 2:05)
*Nhắm: Production Readiness*

**Hình:** tab `/infrastructure`. NGAY TRƯỚC cảnh này chạy: `make feed N=30 && make dispatch` (cần
task đang chạy thật lúc bấm). Bấm **"Simulate crash"** → panel chaos hiện task bị giết.
**✂ JUMP CUT** (chờ 30-90s cho reaper) → panel hiện `task_requeued +Ns` rồi `task_resumed +Ms`.
Chèn overlay "30-90 seconds later" khi dựng.

**Thoại (trước cut):**
> "Now let's be cruel. This button kills an agent in the middle of an
> investigation. No warning. No cleanup."

**Thoại (sau cut):**
> "The reaper noticed the heartbeat stop and put the case back in the queue.
> Another agent picked it up — and resumed from the checkpoint, not from zero.
> This has happened nearly five thousand times in this system. Zero cases
> lost — watch it happen again, live, right now."

*(✅ ĐÃ XÁC MINH `16/08 23:42 UTC` qua `make scorecard`: 8.856 task được re-queue, **4.988 lần resume
từ checkpoint** — số THẬT, còn lớn hơn cả con số 3.201 mà bản draft cũ từng nói (hệ thống đã bị chaos-test
nhiều hơn qua các lần fix bug hôm nay). Nói thẳng "nearly five thousand" là an toàn và đúng. Nếu quay xa
ngày này, chạy lại `make scorecard` để lấy số mới — chỉ tăng theo thời gian, không giảm.)*

---

## CẢNH 5 — Một bộ nhớ, hai người thầy (+ bất tử vùng nếu có) (2:05 – 2:40)
*Nhắm: Agentic Memory Design (trọng số cao nhất) + Production Readiness (nếu có nhịp 3)*

**Hình — hai hoặc ba nhịp:**
1. Tab `/reviews`: nhập tên reviewer (vd `analyst-1`), bấm **Approve** một case. (~7s)
2. Tab `/database`: dán **câu B**, Run → hiện ký ức mới, `salience = 2`. (~13s)
3. *(Chỉ nếu Bước 5 chuẩn bị đã xác nhận 3 region)* Tab `/infrastructure`, panel **Multi-region**:
   chỉ vào badge **"3 regions · region"** và các region chip. (~10s)

**Thoại:**
> "When a human decides an escalated case — one at a time, or hundreds at
> once in bulk — the fleet remembers it.
> Embedded into the same vector memory, pinned at maximum salience —
> decay can never erase what a human taught it. One memory, two teachers."

**Thoại (chỉ nếu có nhịp 3):**
> "And this brain now spans three AWS regions, surviving a full region
> failure with zero data loss. Lose an entire region — the fleet keeps deciding."

*(Câu "one at a time, or hundreds at once in bulk" là điểm mới so với bản v4 — hôm nay đã fix xong
1 bug thật: trước đây bulk-approve KHÔNG dạy được bộ nhớ, chỉ ghi log, giờ mỗi quyết định hàng loạt
cũng được ghim vào case_memory giống hệt single-review. Nói câu này CHỈ KHI bạn đã deploy bản fix mới
nhất lên Lambda — kiểm tra `hivemind-dev-dashboard-api` đang chạy version mới nhất trước khi quay.)*

*(Panel multi-region đọc trực tiếp từ CockroachDB qua `/control/regions` — không phải hình vẽ. Chỉ
quay nhịp 3 SAU KHI `make regions` xác nhận đúng 3 region + survival "region".)*

---

## CẢNH 6 — Vận hành như sản phẩm + kết (2:40 – 2:55)
*Nhắm: Production Readiness*

**Hình:** tab GitHub Actions toàn xanh. Rê qua chuỗi Build → Staging → Smoke → Canary. Đứng hình
cuối: overlay URL dashboard + repo (2 giây).

**Thoại:**
> "And it ships like a product. Every push: a hundred-plus pipeline
> invariants, security scans, then a canary — ten percent of traffic,
> five minutes under CloudWatch, promote or auto-rollback.
> HiveMind — agentic memory on CockroachDB and AWS.
> Everything you just saw is public. Come break it."

*(Giữ nguyên kỹ thuật từ v4: KHÔNG nói số invariant cụ thể — chèn 1-2 giây terminal chạy
`bash test/integration/pipeline_test.sh` làm B-roll, số thật (hiện tại 117, sẽ tăng theo thời gian)
tự hiện trên màn hình, thoại không thể nói sai vì không nói số.)*

---

## SAU KHI QUAY

1. Dựng: cắt jump cut cảnh 4 + overlay "30-90 seconds later", xuất 1080p, **check < 3:00**.
2. Upload YouTube (unlisted là đủ, tuỳ thể lệ).
3. Nếu đã bật multi-region ở Bước 5 — **tắt lại ngay** để đỡ phí:
   ```bash
   ./scripts/multi-region.sh disable aws-ap-southeast-3 aws-ap-south-1
   ```
4. Commit `evidence/recording-day/` (từ Bước 6) — có video + evidence cùng ngày là điểm cộng cho
   giám khảo đối chiếu số liệu.
5. Cập nhật link video vào `SUBMISSION.md`, push commit cuối.

## PHAO CỨU SINH

| Sự cố khi quay | Xử lý |
|---|---|
| `make regions` sau khi enable vẫn báo 1 region / `zone` | CockroachDB Cloud console chưa provision xong — đợi thêm. Nếu vẫn kẹt: quay cảnh 5 chỉ với nhịp 1+2, bỏ nhịp 3 và câu "spans three AWS regions". |
| "Simulate crash" báo không có task đang chạy | `make feed N=30 && make dispatch`, đợi 15 giây, bấm lại. |
| Reaper lâu (>2 phút) | Cứ để máy quay chạy — cắt lúc dựng; reaper theo lịch 30s–1 phút. |
| Review Queue trống (cảnh 5) | `make feed N=40 && make dispatch`, đợi 2 phút — PaySim luôn sinh case escalate. |
| Câu B trả 0 dòng sau khi Approve | F5 đợi 3 giây rồi chạy lại — ghi memory mất ~1 giây. |
| Decision trace không có memory recall | Chọn case `fraud` khác, hoặc nạp thêm case trước để có nhiều memory hơn để gợi nhớ. |
| Hàng đợi cạn ở cảnh 1 | `make feed N=40 && make dispatch`. |
| Cảnh 2 số quá nhỏ, trông không thuyết phục (do DB mới build lại) | Nạp thêm nhiều case hơn trước khi quay (Bước 3), hoặc dùng câu thoại an toàn đã viết sẵn (không nói số cụ thể). |
| Nói vấp | Quay lại riêng cảnh đó, ghép sau. |

## CÁC CON SỐ CẦN NÓI ĐÚNG (đối chiếu evidence trước khi quay, đừng chỉ tin file này)

- **100% / 100%**: traceable → tự quyết · untraceable → escalate (cảnh 3 — hình nói thay, luôn đúng
  vì SQL chạy live, không phụ thuộc số lượng data).
- **Memory** (cảnh 2): ✅ đã xác minh `16/08 23:42 UTC` qua `make scorecard` — 118 memory / 4.321 case,
  trung bình 2.91 recall/investigation, $0.00023/case. Thoại "a hundred eighteen... over four thousand...
  two hundredths of a cent" đã đúng, cứ nói thẳng. Chỉ chạy lại `make scorecard` nếu quay cách xa mốc
  thời gian trên nhiều ngày (số chỉ tăng, không giảm, nên không lo bị nói sai theo hướng phóng đại).
- **Chaos/resume** (cảnh 4): ✅ đã xác minh — 8.856 re-queue, **4.988 resume** từ checkpoint. Thoại
  "nearly five thousand times" đã đúng và lớn hơn cả số "3.201" của bản draft cũ. Không cần né số nữa.
- **Pipeline invariants** (cảnh 6): KHÔNG nói số cụ thể — hiện tại 117 (tăng dần theo thời gian khi thêm
  coverage mới, đọc lại `pipeline_test.sh` lúc quay, đừng hardcode số này). Terminal B-roll thay lời.
- **Multi-region** (cảnh 5 nhịp 3): ✅ đã bật, 3 vùng (Singapore primary + Mumbai + Jakarta thay Sydney),
  `survival_goal: "region"` xác nhận qua `make regions`. Nói thẳng "three AWS regions" — chỉ chạy lại
  `make regions` để xác nhận lại nếu bạn destroy/rebuild hạ tầng sau mốc `16/08 23:42 UTC`.
- **Bulk review dạy bộ nhớ** (cảnh 5): chỉ nói câu này nếu `dashboard-api` đang chạy bản deploy có fix
  hôm nay (bulk review giờ cũng ghi vào case_memory, trước đây chỉ single-review làm được) — kiểm tra
  lại version đang chạy trên Lambda trước khi quay, đừng giả định vì fix đã có trong code.
