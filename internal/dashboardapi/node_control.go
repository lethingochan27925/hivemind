// node_control.go: dieu khien TUNG thanh phan cua fleet thay vi ca cum.
//
//	POST /control/schedule {service, action: enable|disable} - bat/tat 1 schedule
//	POST /control/invoke   {service}                          - chay 1 function ngay
//
// Dung cho drill-down tung node tren dashboard: xem trang thai va thao tac
// truc tiep tren thanh phan do.
package dashboardapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// invokableServices la cac function duoc phep "Run now" tu dashboard - dung
// bang scheduledServices (fleet). Cac service khac (scoring, dashboard-api)
// khong co ly do van hanh de invoke tay.
func invokable(svc string) bool {
	for _, s := range scheduledServices {
		if s == svc {
			return true
		}
	}
	return false
}

// ScheduleOne bat/tat schedule cua MOT service.
func (s *Server) ScheduleOne(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !controlAllowed(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var body struct {
		Service string `json:"service"`
		Action  string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if !invokable(body.Service) {
		http.Error(w, "service has no schedule", http.StatusBadRequest)
		return
	}
	if body.Action != "enable" && body.Action != "disable" {
		http.Error(w, "action must be enable or disable", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	eb, _, err := awsClients(ctx)
	if err != nil {
		http.Error(w, "failed to init aws clients", http.StatusInternalServerError)
		return
	}

	name := aws.String(ruleName(body.Service))
	if body.Action == "enable" {
		_, err = eb.EnableRule(ctx, &eventbridge.EnableRuleInput{Name: name})
	} else {
		_, err = eb.DisableRule(ctx, &eventbridge.DisableRuleInput{Name: name})
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to %s %s: %v", body.Action, body.Service, err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "ok", "service": body.Service, "action": body.Action})
}

// InvokeService chay 1 function cua fleet ngay lap tuc (async) - "Run now".
func (s *Server) InvokeService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !controlAllowed(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var body struct {
		Service string `json:"service"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if !invokable(body.Service) {
		http.Error(w, "service is not invokable from the control plane", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	_, lc, err := awsClients(ctx)
	if err != nil {
		http.Error(w, "failed to init aws clients", http.StatusInternalServerError)
		return
	}

	_, err = lc.Invoke(ctx, &lambda.InvokeInput{
		FunctionName:   aws.String(functionName(body.Service)),
		Qualifier:      aws.String("live"),
		InvocationType: lambdatypes.InvocationTypeEvent,
		Payload:        []byte("{}"),
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("invoking %s: %v", body.Service, err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "invoked", "service": body.Service})
}
