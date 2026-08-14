package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Tu Go 1.22, net/http.ServeMux panic khi hai route dang ky cung mot pattern.
// Panic do xay ra o init, truoc khi handler dau tien chay - tren Lambda no bien
// thanh 502 cho MOI request, va log chi hien khi da deploy. Test nay keo loi do
// ve truoc luc build.
//
// buildMux nhan nil an toan: dang ky route chi lay method value, khong deref.
func TestBuildMuxHasNoDuplicateRoutes(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("route dang ky trung: %v", r)
		}
	}()

	if buildMux(nil) == nil {
		t.Fatal("buildMux tra ve nil")
	}
}

// Cac route loi cua control plane phai luon ton tai - neu mot ban patch xoa nham
// thi dashboard hong im lang chu khong bao loi.
func TestCoreRoutesRegistered(t *testing.T) {
	mux := buildMux(nil)

	for _, route := range []string{
		"/overview",
		"/control/fleet",
		"/control/dispatch",
		"/control/query",
	} {
		req := httptest.NewRequest(http.MethodGet, route, nil)
		if _, pattern := mux.Handler(req); pattern == "" {
			t.Errorf("route %s chua duoc dang ky", route)
		}
	}
}
