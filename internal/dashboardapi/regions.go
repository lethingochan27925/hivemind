// regions.go: /control/regions - doc va dieu khien cau hinh multi-region cua
// DATABASE (ADD/DROP/SET PRIMARY REGION, survival goal). Chi tang database;
// tang CLUSTER (provision node o region moi) la viec cua CockroachDB Cloud
// console/ccloud vi dung den billing - xem scripts/multi-region.sh.
package dashboardapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

type RegionInfo struct {
	Region  string `json:"region"`
	Primary bool   `json:"primary"`
}

type RegionsStatus struct {
	Database     string       `json:"database"`
	MultiRegion  bool         `json:"multi_region"`
	SurvivalGoal string       `json:"survival_goal"`
	Regions      []RegionInfo `json:"regions"`
}

func (s *Server) RegionsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getRegions(w, r)
	case http.MethodPost:
		s.alterRegion(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// getRegions doc SHOW REGIONS FROM DATABASE theo ten cot (so cot khac nhau
// giua cac version CockroachDB), nen khong scan cung vi tri.
func (s *Server) getRegions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st := RegionsStatus{Regions: []RegionInfo{}}
	_ = s.DB.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&st.Database)

	rows, err := s.DB.Pool.Query(ctx, "SHOW REGIONS FROM DATABASE")
	if err == nil {
		defer rows.Close()
		cols := map[string]int{}
		for i, fd := range rows.FieldDescriptions() {
			cols[string(fd.Name)] = i
		}
		for rows.Next() {
			vals, verr := rows.Values()
			if verr != nil {
				break
			}
			ri := RegionInfo{}
			if i, ok := cols["region"]; ok {
				if v, ok := vals[i].(string); ok {
					ri.Region = v
				}
			}
			if i, ok := cols["primary"]; ok {
				if v, ok := vals[i].(bool); ok {
					ri.Primary = v
				}
			}
			if ri.Region != "" {
				st.Regions = append(st.Regions, ri)
			}
		}
	}
	st.MultiRegion = len(st.Regions) > 0

	// Survival goal chi ton tai khi database da multi-region; loi thi bo qua.
	var dbName, goal string
	if err := s.DB.Pool.QueryRow(ctx, "SHOW SURVIVAL GOAL FROM DATABASE").Scan(&dbName, &goal); err == nil {
		st.SurvivalGoal = goal
	}

	writeJSON(w, st)
}

var regionNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)

// alterRegion chay dung MOT lenh ALTER DATABASE da validate. Region name qua
// regex chat + identifier duoc quote, khong noi suy chuoi tho tu nguoi dung.
func (s *Server) alterRegion(w http.ResponseWriter, r *http.Request) {
	if !controlAllowed(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var body struct {
		Action string `json:"action"`
		Region string `json:"region"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	action := strings.ToLower(strings.TrimSpace(body.Action))
	region := strings.ToLower(strings.TrimSpace(body.Region))

	// Chan action la ngay - khong de request rac mo ket noi database.
	validActions := map[string]bool{
		"add": true, "drop": true, "set_primary": true,
		"survive_region": true, "survive_zone": true,
	}
	if !validActions[action] {
		http.Error(w, "action must be add | drop | set_primary | survive_region | survive_zone", http.StatusBadRequest)
		return
	}

	needsRegion := action == "add" || action == "drop" || action == "set_primary"
	if needsRegion && !regionNameRe.MatchString(region) {
		http.Error(w, "invalid region name", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var dbName string
	if err := s.DB.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName); err != nil {
		http.Error(w, "cannot resolve database name", http.StatusInternalServerError)
		return
	}

	var stmt string
	switch action {
	case "add":
		stmt = fmt.Sprintf(`ALTER DATABASE %q ADD REGION IF NOT EXISTS %q`, dbName, region)
	case "drop":
		stmt = fmt.Sprintf(`ALTER DATABASE %q DROP REGION IF EXISTS %q`, dbName, region)
	case "set_primary":
		stmt = fmt.Sprintf(`ALTER DATABASE %q SET PRIMARY REGION %q`, dbName, region)
	case "survive_region":
		stmt = fmt.Sprintf(`ALTER DATABASE %q SURVIVE REGION FAILURE`, dbName)
	case "survive_zone":
		stmt = fmt.Sprintf(`ALTER DATABASE %q SURVIVE ZONE FAILURE`, dbName)
	default:
		http.Error(w, "action must be add | drop | set_primary | survive_region | survive_zone", http.StatusBadRequest)
		return
	}

	if _, err := s.DB.Pool.Exec(ctx, stmt); err != nil {
		// Loi tu CockroachDB rat mo ta (vd region chua duoc provision o tang
		// cluster) - tra thang ve UI de nguoi van hanh biet phai lam gi.
		http.Error(w, fmt.Sprintf("region change failed: %v", err), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]string{"status": "ok", "action": action, "region": region})
}
