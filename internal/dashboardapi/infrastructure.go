// infrastructure.go: GET /infrastructure - trang Infrastructure & Resilience, doc CloudWatch.
package dashboardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

type ServiceHealth struct {
	Service    string `json:"service"`
	AlarmState string `json:"alarm_state"`
}

type IncidentEvent struct {
	Timestamp   string `json:"timestamp"`
	Service     string `json:"service"`
	Description string `json:"description"`
}

type InfrastructureResponse struct {
	Services  []ServiceHealth `json:"services"`
	Incidents []IncidentEvent `json:"incidents"`
}

var services = []string{
	"agent-worker", "dispatcher", "reaper", "salience-decay",
	"scoring-api", "scoring-python", "dashboard-api",
}

func (s *Server) GetInfrastructure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()

	cwClient, err := getCloudWatchClient(ctx)
	if err != nil {
		http.Error(w, "failed to connect to CloudWatch", http.StatusInternalServerError)
		return
	}

	healths := make([]ServiceHealth, 0, len(services))
	for _, svc := range services {
		alarmName := fmt.Sprintf("hivemind-dev-%s-errors", svc)
		state, err := getAlarmState(ctx, cwClient, alarmName)
		if err != nil {
			state = "UNKNOWN"
		}
		healths = append(healths, ServiceHealth{Service: svc, AlarmState: state})
	}

	incidents, err := s.queryIncidents(ctx)
	if err != nil {
		incidents = []IncidentEvent{}
	}

	resp := InfrastructureResponse{
		Services:  healths,
		Incidents: incidents,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func getCloudWatchClient(ctx context.Context) (*cloudwatch.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return cloudwatch.NewFromConfig(cfg), nil
}

func getAlarmState(ctx context.Context, client *cloudwatch.Client, alarmName string) (string, error) {
	out, err := client.DescribeAlarms(ctx, &cloudwatch.DescribeAlarmsInput{
		AlarmNames: []string{alarmName},
	})
	if err != nil {
		return "", err
	}
	if len(out.MetricAlarms) == 0 {
		return "UNKNOWN", nil
	}
	return string(out.MetricAlarms[0].StateValue), nil
}

func (s *Server) queryIncidents(ctx context.Context) ([]IncidentEvent, error) {
	rows, err := s.DB.Pool.Query(ctx, `
		SELECT created_at, agent_id, reasoning
		FROM audit_log
		WHERE action = 'task_requeued'
		  AND created_at >= now() - INTERVAL '24 hours'
		ORDER BY created_at DESC
		LIMIT 20
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []IncidentEvent
	for rows.Next() {
		var e IncidentEvent
		var reasoning *string
		var createdAt time.Time
		if err := rows.Scan(&createdAt, &e.Service, &reasoning); err != nil {
			return nil, err
		}
		e.Timestamp = createdAt.Format(time.RFC3339)
		if reasoning != nil {
			e.Description = *reasoning
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

var _ = cwtypes.StateValueOk
