package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/application"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/httpapi"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/storage/filecase"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:19081", "监听地址")
	dataDir := flag.String("data-dir", "", "档案数据目录")
	self := flag.Bool("selfcheck", false, "执行完整自检")
	timeout := flag.Duration("selfcheck-timeout", 8*time.Second, "自检超时")
	flag.Parse()
	if flag.Lookup("addr").Value.String() == "127.0.0.1:19081" {
		if p := os.Getenv("PORT"); p != "" {
			*addr = "127.0.0.1:" + p
		}
	}
	dir := strings.TrimSpace(*dataDir)
	if dir == "" {
		dir = os.Getenv("CHAMBER_DATA_DIR")
	}
	if dir == "" {
		dir = "./.refill-private/data"
	}
	repo, e := filecase.New(dir)
	if e != nil {
		log.Fatal(e)
	}
	app := application.New(repo)
	srv := &http.Server{Addr: *addr, Handler: httpapi.RequestID(httpapi.WithJSON(httpapi.New(app).Handler()))}
	if *self {
		if e := runSelfCheck(*addr, srv, *timeout); e != nil {
			log.Fatal(e)
		}
		return
	}
	go func() {
		if e := srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			log.Fatal(e)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
func runSelfCheck(addr string, srv *http.Server, timeout time.Duration) error {
	go srv.ListenAndServe()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, e := http.Get("http://" + addr + "/api/v1/commissioning-cases/not-found")
		if e == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	c := http.Client{Timeout: 2 * time.Second}
	post := func(path, body string, h map[string]string) ([]byte, error) {
		req, _ := http.NewRequest("POST", "http://"+addr+path, strings.NewReader(body))
		for k, v := range h {
			req.Header.Set(k, v)
		}
		resp, e := c.Do(req)
		if e != nil {
			return nil, e
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("自检 %s 返回 %d: %s", path, resp.StatusCode, string(b))
		}
		return b, nil
	}
	read := func(b []byte) (string, int64, error) {
		var v struct {
			CaseID          string `json:"caseId"`
			ExpectedVersion int64  `json:"expectedVersion"`
		}
		if e := json.Unmarshal(b, &v); e != nil {
			return "", 0, e
		}
		return v.CaseID, v.ExpectedVersion, nil
	}
	b, e := post("/api/v1/commissioning-cases", `{"zoneCode":"A-01","collectionCategory":"书画","ownerName":"负责人"}`, map[string]string{})
	if e != nil {
		return e
	}
	id, v, e := read(b)
	if e != nil {
		return e
	}
	h := map[string]string{"X-Expected-Version": fmt.Sprint(v)}
	for _, step := range []struct{ path, body string }{{"/baseline", `{"temperatureMin":18,"temperatureMax":24,"humidityMin":45,"humidityMax":55,"samplingIntervalMinutes":180,"minimumObservationCount":2}`}, {"/plan", `{"deviceLabel":"HVAC-1","controlMode":"auto","setpointTemperature":21,"setpointHumidity":50,"trialDurationHours":1,"submittedBy":"负责人"}`}, {"/start", `{}`}} {
		b, e = post("/api/v1/commissioning-cases/"+id+step.path, step.body, h)
		if e != nil {
			return e
		}
		_, v, e = read(b)
		if e != nil {
			return e
		}
		h["X-Expected-Version"] = fmt.Sprint(v)
	}
	now := time.Now().UTC()
	for i, t := range []time.Time{now.Add(-2 * time.Hour), now} {
		body := fmt.Sprintf(`{"sequence":%d,"observedAt":%q,"temperature":21,"humidity":50,"deviceStatus":"normal","recordedBy":"现场"}`, i+1, t.Format(time.RFC3339))
		b, e = post("/api/v1/commissioning-cases/"+id+"/observations", body, h)
		if e != nil {
			return e
		}
		_, v, e = read(b)
		if e != nil {
			return e
		}
		h["X-Expected-Version"] = fmt.Sprint(v)
	}
	resp, e := c.Get("http://" + addr + "/api/v1/commissioning-cases/" + id + "/review-package")
	if e != nil {
		return e
	}
	packageBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("自检复核资料包返回 %d", resp.StatusCode)
	}
	var reviewPackage struct {
		PackageFingerprint string `json:"packageFingerprint"`
	}
	if json.Unmarshal(packageBody, &reviewPackage) != nil || reviewPackage.PackageFingerprint == "" {
		return fmt.Errorf("自检复核资料包无效")
	}
	review := fmt.Sprintf(`{"reviewerName":"复核员","decision":"approve","comment":"通过","reviewedVersion":%d,"packageFingerprint":%q}`, v, reviewPackage.PackageFingerprint)
	b, e = post("/api/v1/commissioning-cases/"+id+"/review", review, h)
	if e != nil {
		return e
	}
	_, v, e = read(b)
	if e != nil {
		return e
	}
	h["X-Expected-Version"] = fmt.Sprint(v)
	b, e = post("/api/v1/commissioning-cases/"+id+"/activate", `{}`, h)
	if e != nil {
		return e
	}
	var final struct {
		Permit *struct {
			PermitCode string `json:"permitCode"`
		} `json:"permit"`
	}
	if json.Unmarshal(b, &final) != nil || final.Permit == nil {
		return fmt.Errorf("自检未签发许可")
	}
	resp, e = c.Get("http://" + addr + "/api/v1/permits/" + final.Permit.PermitCode + "/validation")
	if e != nil {
		return e
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("自检许可验证失败")
	}
	_ = srv.Shutdown(context.Background())
	return nil
}
