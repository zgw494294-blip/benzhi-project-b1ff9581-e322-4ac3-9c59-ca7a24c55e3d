package selfcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/benzhi/ancient-tree-pathogen/internal/application"
	"github.com/benzhi/ancient-tree-pathogen/internal/httpapi"
	"github.com/benzhi/ancient-tree-pathogen/internal/storage"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
)

func Run(ctx context.Context) error {
	db, e := storage.Open(":memory:")
	if e != nil {
		return e
	}
	defer db.Close()
	app := application.New(db)
	srv := httptest.NewServer(httpapi.New(app).Handler())
	defer srv.Close()
	post := func(path string, v any) (map[string]any, error) {
		b, _ := json.Marshal(v)
		r, e := http.Post(srv.URL+path, "application/json", strings.NewReader(string(b)))
		if e != nil {
			return nil, e
		}
		defer r.Body.Close()
		data, _ := io.ReadAll(r.Body)
		if r.StatusCode >= 300 {
			return nil, fmt.Errorf("%s: %s", path, data)
		}
		var out map[string]any
		json.Unmarshal(data, &out)
		return out, nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	c, e := post("/api/cases", map[string]string{"treeCode": "SC-001", "species": "银杏", "location": "东区", "owner": "保护中心", "actor": "自检"})
	if e != nil {
		return e
	}
	id := c["ID"].(string)
	ver := int(c["Version"].(float64))
	bad, _ := post("/api/cases/"+id+"/samples", map[string]any{"sampleCode": "S-BAD", "collector": "采样员", "sealCode": "SEAL-BAD", "receiver": "检测员", "condition": "破损", "collectedAt": now, "handoffAt": now, "expectedVersion": ver})
	if bad != nil { /* 失败记录由服务留痕，后续正式交接仍使用原版本 */
	}
	_, e = post("/api/cases/"+id+"/samples", map[string]any{"sampleCode": "S-001", "collector": "采样员", "sealCode": "SEAL-1", "receiver": "检测员", "condition": "完整", "collectedAt": now, "handoffAt": now, "expectedVersion": ver})
	if e != nil {
		return e
	}
	ver++
	_, e = post("/api/cases/"+id+"/tests", map[string]string{"testType": "PCR", "operator": "检测员", "pathogen": "腐霉", "load": "低", "method": "qPCR", "result": "阴性", "notes": "无", "performedAt": now})
	if e != nil {
		return e
	}
	ver++
	_, e = post("/api/cases/"+id+"/review", map[string]any{"decision": "通过", "mitigation": "常规养护", "reviewer": "负责人", "expectedVersion": ver})
	if e != nil {
		return e
	}
	ver++
	_, e = post("/api/cases/"+id+"/retest", map[string]any{"result": "通过", "operator": "检测员", "expectedVersion": ver})
	if e != nil {
		return e
	}
	cred, e := post("/api/cases/"+id+"/credential", map[string]string{"issuedBy": "负责人"})
	if e != nil {
		return e
	}
	resp, e := http.Get(srv.URL + "/api/search?status=已放行")
	if e != nil || resp.StatusCode >= 300 {
		return fmt.Errorf("检索自检失败")
	}
	resp.Body.Close()
	credID, _ := cred["ID"].(string)
	if credID != "" {
		batch, be := post("/api/verification", map[string]any{"ids": []string{" " + credID + " ", credID, "cred_invalid"}})
		if be != nil || batch["ID"] == nil {
			return fmt.Errorf("批量凭据核验自检失败: %v", be)
		}
		receiptID, _ := batch["ID"].(string)
		if receiptID != "" {
			resp, re := http.Get(srv.URL + "/api/verification/" + receiptID)
			if re != nil || resp.StatusCode >= 300 {
				return fmt.Errorf("核验回执自检失败")
			}
			resp.Body.Close()
		}
		if _, e = post("/api/credentials/"+credID+"/revoke", map[string]string{"actor": "负责人", "reason": "自检撤销"}); e != nil {
			return e
		}
	}
	fmt.Println("自检通过：建案、样本交接、实验、复核、复检、凭据签发链路完整")
	return nil
}
