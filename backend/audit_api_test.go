package main

import (
 "path/filepath"
 "os"
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
 for _,route:=range []string{"/audit","/audit.js"}{
  if w:=req(c,"GET",route,reader,false);w.Code!=401{t.Fatalf("reader audit UI exposed: %s %d",route,w.Code)}
  if w:=req(c,"GET",route,admin,true);w.Code!=200{t.Fatalf("admin UI denied: %s %d",route,w.Code)}
 }
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

func TestFixedReportHTTPRealPostgres(t *testing.T){
 dsn:=os.Getenv("MACKNA_LAB_DSN")
 if dsn==""{t.Skip("CI PostgreSQL 16 not connected")}
 t.Setenv("MACKNA_PG_LAB_ENABLED","1")
 t.Setenv("MACKNA_PG_LAB_DSN",dsn)
 c,reader,_:=setup(t)
 for _,name:=range []string{"connections","table-stats","index-stats","locks","database-stats","settings","extensions"}{
  route:="/api/v1/postgres/lab/report?name="+name
  resp:=req(c,"GET",route,reader,false)
  if resp.Code!=200 || !strings.Contains(resp.Body.String(),"\"columns\"") {
   t.Fatalf("%s: HTTP %d %s",route,resp.Code,resp.Body.String())
  }
 }
 for _,route:=range []string{
  "/api/v1/postgres/lab/report?name=secret",
  "/api/v1/postgres/lab/report?name=connections%3BDROP+TABLE+x",
 }{
  resp:=req(c,"GET",route,reader,false)
  if resp.Code!=404{t.Fatalf("unknown report accepted: %d %s",resp.Code,resp.Body.String())}
 }
 resp:=req(c,"GET","/api/v1/postgres/lab/report?name=connections&sql=SELECT+1",reader,false)
 if resp.Code!=400{t.Fatalf("unexpected report parameters accepted: %d",resp.Code)}
}
