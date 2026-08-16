// Package cache: mot cache TTL generic cho gia tri ma mot Lambda container
// (song qua nhieu request) tinh lai hiem khi - mot cuoc goi API ben ngoai,
// mot round-trip AWS SDK - va muon moi request dong thoi trong cua so TTL
// dung chung thay vi lap lai.
//
// Hinh dang nay - mot mutex, mot gia tri da cache, va "con hieu luc bao lau" -
// da xuat hien doc lap ba lan (chi tieu Cost Explorer trong
// internal/dashboardapi/cloudcost.go, don gia Bedrock trong
// internal/pricing/pricing.go, va proxy GitHub Actions trong
// internal/dashboardapi/pipeline.go) truoc khi duoc rut ra day; mot the tu -
// config.Load() trong internal/dashboardapi/cost.go - dung cung hinh dang do
// nhung khong bao gio het han (Forever).
package cache

import (
	"sync"
	"time"
)

// Forever la mot TTL du lon de gia tri da cache, tren thuc te, khong bao gio
// bi coi la cu - danh cho gia tri chi can tinh lai sau khi lan thu truoc that
// bai (xem cache cua config trong internal/dashboardapi/cost.go).
const Forever = 100 * 365 * 24 * time.Hour

// TTL cache mot gia tri duoi mot mutex duoc giu XUYEN SUOT luc tinh lai. Day
// la lua chon co chu dich, khong phai so sot: moi caller den dung luc cache
// vua mien lai bi khoa tren cung mutex thay vi tu minh tinh lai mot ban sao,
// va duoc mo khoa thang vao cache vua tuoi ngay khi nguoi thang cuoc xong -
// cung hieu ung mot singleflight.Group se cho voi mot cache-key duy nhat,
// nhung it thanh phan hon.
type TTL[T any] struct {
	defaultTTL time.Duration

	mu        sync.Mutex
	val       T
	expiresAt time.Time
	set       bool
}

// NewTTL tao mot cache ma moi gia tri, mot khi duoc ghi, duoc coi la con tuoi
// trong defaultTTL tru khi Fetch tu ghi de (xem doc cua Fetch).
func NewTTL[T any](defaultTTL time.Duration) *TTL[T] {
	return &TTL[T]{defaultTTL: defaultTTL}
}

// Fetch tinh lai gia tri. ttl == 0 cache ket qua trong defaultTTL cua cache;
// ttl > 0 ghi de TTL do rieng cho ket qua nay (ly do cai nay ton tai: gia
// fallback "static" ngan han cua pricing.go - mot lan Pricing API that bai
// khong duoc phep ghim gia doan mo suot ca cua so 12h); ttl < 0 nghia la "day
// la ket qua hop le, tra cho caller, nhung DUNG cache no" (phan hoi "Cost
// Explorer unavailable" cua cloudcost.go, hoac mot loi 4xx/5xx tu GitHub duoc
// pipeline.go chuyen tiep nguyen ven - that nhung khong danh cho nhung
// caller sau tiep tuc nhan). Mot error khac nil khong bao gio duoc cache,
// bat ke ttl.
type Fetch[T any] func() (value T, ttl time.Duration, err error)

// Get tra ve gia tri da cache neu con tuoi, neu khong thi chay fetch duoi
// cung mot khoa va cap nhat cache theo ttl no tra ve (xem Fetch).
func (c *TTL[T]) Get(fetch Fetch[T]) (T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.set && time.Now().Before(c.expiresAt) {
		return c.val, nil
	}

	v, ttl, err := fetch()
	if err != nil {
		var zero T
		return zero, err
	}
	switch {
	case ttl < 0:
		c.set = false
	case ttl == 0:
		c.val, c.expiresAt, c.set = v, time.Now().Add(c.defaultTTL), true
	default:
		c.val, c.expiresAt, c.set = v, time.Now().Add(ttl), true
	}
	return v, nil
}
