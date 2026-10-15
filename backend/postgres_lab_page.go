package main

import (
 "embed"
 "net/http"
 "os"
)

//go:embed postgres_lab.html postgres_lab.js
var postgresLabHTML embed.FS

func (c *controller) servePostgresLabScript(w http.ResponseWriter,r *http.Request){
 if os.Getenv("MACKNA_PG_LAB_ENABLED")!="1"{http.NotFound(w,r);return}
 content,err:=postgresLabHTML.ReadFile("postgres_lab.js")
 if err!=nil{http.Error(w,"script unavailable",500);return}
 w.Header().Set("Content-Type","text/javascript; charset=utf-8")
 _,_=w.Write(content)
}

func (c *controller) servePostgresLabPage(w http.ResponseWriter,r *http.Request){
 if os.Getenv("MACKNA_PG_LAB_ENABLED")!="1"{
  http.NotFound(w,r);return
 }
 content,err:=postgresLabHTML.ReadFile("postgres_lab.html")
 if err!=nil{http.Error(w,"template unavailable",500);return}
 w.Header().Set("Content-Type","text/html; charset=utf-8")
 _,_=w.Write(content)
}
