package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/application"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/storage/filecase"
)

func TestExtendedPublicWorkflow(t *testing.T) {
	repo, err := filecase.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(application.New(repo)).Handler())
	defer server.Close()
	client := server.Client()

	var c commissioning.CommissioningCase
	doJSON(t, client, http.MethodPost, server.URL+"/api/v1/commissioning-cases", `{"zoneCode":" a-1 ","collectionCategory":" 书画 ","ownerName":" 负责人 "}`, nil, http.StatusCreated, &c)
	if c.ZoneCode != "A-1" {
		t.Fatalf("zoneCode=%s", c.ZoneCode)
	}
	version := c.ExpectedVersion
	headers := func() map[string]string {
		return map[string]string{"X-Expected-Version": fmt.Sprint(version), "Idempotency-Key": fmt.Sprintf("step-%d", version)}
	}

	doJSON(t, client, http.MethodPatch, server.URL+"/api/v1/commissioning-cases/"+c.CaseID, `{"zoneCode":" b-2 ","collectionCategory":"陶瓷","ownerName":"管理员"}`, headers(), http.StatusOK, &c)
	version = c.ExpectedVersion
	baseline := `{"temperatureMin":18,"temperatureMax":24,"humidityMin":40,"humidityMax":60,"samplingIntervalMinutes":180,"minimumObservationCount":2}`
	doJSON(t, client, http.MethodPost, server.URL+"/api/v1/commissioning-cases/"+c.CaseID+"/baseline", baseline, headers(), http.StatusOK, &c)
	version = c.ExpectedVersion
	doJSON(t, client, http.MethodPost, server.URL+"/api/v1/commissioning-cases/"+c.CaseID+"/baseline/revoke", `{"reason":"数据源更新","operator":"管理员"}`, headers(), http.StatusOK, &c)
	version = c.ExpectedVersion
	doJSON(t, client, http.MethodPost, server.URL+"/api/v1/commissioning-cases/"+c.CaseID+"/baseline", baseline, headers(), http.StatusOK, &c)
	if c.Baseline == nil || c.Baseline.Revision != 2 {
		t.Fatalf("baseline=%#v", c.Baseline)
	}
	version = c.ExpectedVersion
	plan := `{"deviceLabel":"HVAC-1","controlMode":"auto","setpointTemperature":21,"setpointHumidity":50,"trialDurationHours":1,"submittedBy":"提交人"}`
	doJSON(t, client, http.MethodPost, server.URL+"/api/v1/commissioning-cases/"+c.CaseID+"/plan", plan, headers(), http.StatusOK, &c)
	version = c.ExpectedVersion
	revision := `{"deviceLabel":"HVAC-2","controlMode":"auto","setpointTemperature":22,"setpointHumidity":50,"trialDurationHours":1,"submittedBy":"新提交人","reason":"更换设备"}`
	doJSON(t, client, http.MethodPatch, server.URL+"/api/v1/commissioning-cases/"+c.CaseID+"/plan", revision, headers(), http.StatusOK, &c)
	if len(c.PlanHistory) != 2 || c.PlanHistory[1].Revision != 2 {
		t.Fatalf("planHistory=%#v", c.PlanHistory)
	}
	version = c.ExpectedVersion
	doJSON(t, client, http.MethodPost, server.URL+"/api/v1/commissioning-cases/"+c.CaseID+"/start", `{}`, headers(), http.StatusOK, &c)
	version = c.ExpectedVersion
	now := time.Now().UTC()
	batch := fmt.Sprintf(`{"observations":[{"observationId":"first","sequence":1,"observedAt":%q,"temperature":21,"humidity":50,"deviceStatus":"normal","recordedBy":"现场"},{"observationId":"second","sequence":2,"observedAt":%q,"temperature":21,"humidity":50,"deviceStatus":"normal","recordedBy":"现场"}]}`, now.Add(-2*time.Hour).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	batchHeaders := headers()
	doJSON(t, client, http.MethodPost, server.URL+"/api/v1/commissioning-cases/"+c.CaseID+"/observations/batch", batch, batchHeaders, http.StatusOK, &c)
	batchVersion := c.ExpectedVersion
	var replay commissioning.CommissioningCase
	doJSON(t, client, http.MethodPost, server.URL+"/api/v1/commissioning-cases/"+c.CaseID+"/observations/batch", batch, batchHeaders, http.StatusOK, &replay)
	if replay.ExpectedVersion != batchVersion || len(replay.Observations) != 2 {
		t.Fatalf("批量观测幂等重放异常: %#v", replay)
	}
	version = c.ExpectedVersion
	var ledger commissioning.DeviationLedger
	doJSON(t, client, http.MethodGet, server.URL+"/api/v1/commissioning-cases/"+c.CaseID+"/deviations?page=1&pageSize=10", "", nil, http.StatusOK, &ledger)
	if ledger.Total != 0 || ledger.StatusCounts[commissioning.DeviationOpen] != 0 {
		t.Fatalf("deviation ledger=%#v", ledger)
	}

	var pkg commissioning.ReviewPackage
	doJSON(t, client, http.MethodGet, server.URL+"/api/v1/commissioning-cases/"+c.CaseID+"/review-package", "", nil, http.StatusOK, &pkg)
	review := fmt.Sprintf(`{"reviewerName":"复核员","decision":"approve","comment":"通过","reviewedVersion":%d,"packageFingerprint":%q}`, version, pkg.PackageFingerprint)
	doJSON(t, client, http.MethodPost, server.URL+"/api/v1/commissioning-cases/"+c.CaseID+"/review", review, headers(), http.StatusOK, &c)
	version = c.ExpectedVersion
	doJSON(t, client, http.MethodPost, server.URL+"/api/v1/commissioning-cases/"+c.CaseID+"/activate", `{}`, headers(), http.StatusOK, &c)
	if c.Permit == nil {
		t.Fatal("未签发许可")
	}

	var validation application.PermitValidation
	doJSON(t, client, http.MethodGet, server.URL+"/api/v1/permits/"+c.Permit.PermitCode+"/validation", "", nil, http.StatusOK, &validation)
	if validation.Status != "active" || validation.BaselineRevision != 2 || validation.PlanRevision != 2 || validation.ApprovedBy != "复核员" {
		t.Fatalf("validation=%#v", validation)
	}
	var batchValidation application.PermitBatchValidation
	permitBatch := fmt.Sprintf(`{"permitCodes":[%q,"UNKNOWN"]}`, c.Permit.PermitCode)
	doJSON(t, client, http.MethodPost, server.URL+"/api/v1/permits/validation", permitBatch, nil, http.StatusOK, &batchValidation)
	if len(batchValidation.Items) != 2 || batchValidation.Items[0].Status != "active" || batchValidation.Items[1].Status != "not_found" || batchValidation.Summary.Active != 1 || batchValidation.Summary.NotFound != 1 {
		t.Fatalf("batch validation=%#v", batchValidation)
	}
	var list application.CaseList
	doJSON(t, client, http.MethodGet, server.URL+"/api/v1/commissioning-cases?state=Activated&zoneCode=B-2&page=1&pageSize=10", "", nil, http.StatusOK, &list)
	if list.Total != 1 || len(list.Items) != 1 || list.StateCounts[commissioning.Activated] != 1 {
		t.Fatalf("list=%#v", list)
	}
}

func doJSON(t *testing.T, client *http.Client, method, url, body string, headers map[string]string, wantStatus int, target any) {
	t.Helper()
	request, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != wantStatus {
		t.Fatalf("%s %s: status=%d body=%s", method, url, response.StatusCode, payload)
	}
	if target != nil && len(payload) > 0 {
		if err := json.Unmarshal(payload, target); err != nil {
			t.Fatalf("响应解码失败: %v body=%s", err, payload)
		}
	}
}
