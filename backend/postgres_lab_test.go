package main

import (
 "net/http"
 "net/http/httptest"
 "testing"
)

func TestPostgresLabDisabledByDefault(t *testing.T){
 t.Setenv("MACKNA_PG_LAB_ENABLED","")
 req:=httptest.NewRequest(http.MethodGet,"/api/v1/postgres/lab/overview",nil)
 rec:=httptest.NewRecorder()
 (&controller{}).servePostgresLab(rec,req)
 if rec.Code!=http.StatusNotFound{t.Fatalf("expected 404, got %d",rec.Code)}
}
func TestPostgresLabRejectsProductionDSN(t *testing.T){
 t.Setenv("MACKNA_PG_LAB_ENABLED","1")
 t.Setenv("MACKNA_PG_LAB_DSN","postgres://mackna_reader:secret@127.0.0.1:5432/imortais?sslmode=disable")
 req:=httptest.NewRequest(http.MethodGet,"/api/v1/postgres/lab/overview",nil)
 rec:=httptest.NewRecorder()
 (&controller{}).servePostgresLab(rec,req)
 if rec.Code!=http.StatusServiceUnavailable{t.Fatalf("expected 503, got %d",rec.Code)}
}
func TestPostgresLabRejectsPrivilegedRole(t *testing.T){
 t.Setenv("MACKNA_PG_LAB_ENABLED","1")
 t.Setenv("MACKNA_PG_LAB_DSN","postgres://postgres:secret@127.0.0.1:5432/mackna_lab?sslmode=disable")
 req:=httptest.NewRequest(http.MethodGet,"/api/v1/postgres/lab/overview",nil)
 rec:=httptest.NewRecorder()
 (&controller{}).servePostgresLab(rec,req)
 if rec.Code!=http.StatusServiceUnavailable{t.Fatalf("expected 503, got %d",rec.Code)}
}
