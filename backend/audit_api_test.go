package main

import (
 "path/filepath"
 "strings"
 "testing"

 "github.com/cocoputiz-sudo/mackna-cloud/backend/internal/audit"
)
func TestAdminAuditAPI(t *testing.T){
 c,reader,admin:=setup(t)
 l,err:=audit.Open(filepath.Join(t.TempDir(),"audit.jsonl"),4096)
 if err!=nil{t.Fatal(err)}
 defer l.Close();c.Audit=l
 if err:=l.Write(audit.Event{Action:"pg_lab_read_attempt",TokenID:"id-only",Path:"/api/v1/postgres/lab/report",Status:0});err!=nil{t.Fatal(err)}
 if w:=req(c,"GET","/api/v1/audit",reader,false);w.Code!=401{t.Fatalf("reader audit exposed: %d",w.Code)}
 if w:=req(c,"POST","/api/v1/audit",admin,false);w.Code!=405{t.Fatalf("audit write allowed: %d",w.Code)}
 if w:=req(c,"GET","/api/v1/audit?limit=1000",admin,false);w.Code!=400{t.Fatalf("invalid limit allowed: %d",w.Code)}
 if w:=req(c,"GET","/api/v1/audit?ignored=true",admin,false);w.Code!=400{t.Fatalf("unknown query allowed: %d",w.Code)}
 w:=req(c,"GET","/api/v1/audit?limit=5",admin,false)
 if w.Code!=200||!strings.Contains(w.Body.String(),"pg_lab_read_attempt"){t.Fatalf("audit unavailable: %d %s",w.Code,w.Body.String())}
 if strings.Contains(w.Body.String(),"authorization")||strings.Contains(w.Body.String(),"mkn_"){t.Fatal("auth details leaked")}
}
func TestFixedReportsRequireAuthorization(t *testing.T){
 c,reader,_:=setup(t)
 if w:=req(c,"GET","/api/v1/postgres/lab/report?name=connections","",false);w.Code!=401{t.Fatalf("unauthorized report: %d",w.Code)}
 t.Setenv("MACKNA_PG_LAB_ENABLED","0")
 if w:=req(c,"GET","/api/v1/postgres/lab/report?name=connections",reader,false);w.Code!=404{t.Fatalf("disabled lab accepted: %d",w.Code)}
}
