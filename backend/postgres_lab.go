package main

import (
 "context"
 "errors"
 "net/http"
 "os"
 "strconv"
 "strings"
 "time"

 "github.com/cocoputiz-sudo/mackna-cloud/backend/internal/postgres"
 "github.com/jackc/pgx/v5"
)

// Phase A is opt-in and hard-limited to the disposable mackna_lab database.
// It does not discover Docker databases, credentials, or production DSNs.
func (c *controller) servePostgresLab(w http.ResponseWriter,r *http.Request){
 if os.Getenv("MACKNA_PG_LAB_ENABLED")!="1" {
  jsonResponse(w,http.StatusNotFound,map[string]string{"error":"postgres_lab_disabled"});return
 }
 dsn:=os.Getenv("MACKNA_PG_LAB_DSN")
 if dsn=="" {jsonResponse(w,503,map[string]string{"error":"lab_not_configured"});return}
 cfg,err:=pgx.ParseConfig(dsn)
 if err!=nil||cfg.Database!="mackna_lab"||cfg.User!="mackna_reader" {
  jsonResponse(w,503,map[string]string{"error":"lab_configuration_rejected"});return
 }
 ctx,cancel:=context.WithTimeout(r.Context(),15*time.Second);defer cancel()
 reader,err:=postgres.Open(ctx,dsn)
 if err!=nil{jsonResponse(w,503,map[string]string{"error":"lab_unavailable"});return}
 defer reader.Close()
 path:=strings.TrimPrefix(r.URL.Path,"/api/v1/postgres/lab/")
 q:=r.URL.Query()
 var result any
 switch path{
 case "overview":
  if len(q)!=0 {jsonResponse(w,400,map[string]string{"error":"unexpected_query"});return}
  result,err=reader.Overview(ctx)
 case "schemas":
  if len(q)!=0 {jsonResponse(w,400,map[string]string{"error":"unexpected_query"});return}
  result,err=reader.Schemas(ctx)
 case "tables":
  if !queryOnly(q,"schema"){jsonResponse(w,400,map[string]string{"error":"invalid_query"});return}
  result,err=reader.Tables(ctx,q.Get("schema"))
 case "columns":
  if !queryOnly(q,"schema","table")||!q.Has("table") {jsonResponse(w,400,map[string]string{"error":"invalid_query"});return}
  result,err=reader.Columns(ctx,q.Get("schema"),q.Get("table"))
 case "indexes":
  if !queryOnly(q,"schema","table")||!q.Has("table"){jsonResponse(w,400,map[string]string{"error":"invalid_query"});return}
  result,err=reader.Indexes(ctx,q.Get("schema"),q.Get("table"))
 case "foreign-keys":
  if !queryOnly(q,"schema","table")||!q.Has("table"){jsonResponse(w,400,map[string]string{"error":"invalid_query"});return}
  result,err=reader.ForeignKeys(ctx,q.Get("schema"),q.Get("table"))
 case "rows":
  if !queryOnly(q,"schema","table","limit","after")||!q.Has("table"){jsonResponse(w,400,map[string]string{"error":"invalid_query"});return}
  limit:=100
  if q.Has("limit"){limit,err=strconv.Atoi(q.Get("limit"));if err!=nil||limit<1||limit>postgres.MaxRows{jsonResponse(w,400,map[string]string{"error":"invalid_limit"});return}}
  var after any
  if q.Has("after"){after=q.Get("after")}
  result,err=reader.Rows(ctx,q.Get("schema"),q.Get("table"),after,limit)
 default:
  jsonResponse(w,404,map[string]string{"error":"not_found"});return
 }
 if err!=nil{
  status:=http.StatusBadGateway
  if errors.Is(err,context.DeadlineExceeded){status=http.StatusGatewayTimeout}
  jsonResponse(w,status,map[string]string{"error":"postgres_query_failed"});return
 }
 jsonResponse(w,200,result)
}
