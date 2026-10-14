package main

import (
 "bytes"
 "context"
 "encoding/json"
 "io"
 "net/http"
 "net/http/httptest"
 "path/filepath"
 "strings"
 "testing"

 "github.com/cocoputiz-sudo/mackna-cloud/backend/internal/auth"
)

type roundTripFunc func(*http.Request)(*http.Response,error)
func (f roundTripFunc) RoundTrip(r *http.Request)(*http.Response,error){return f(r)}
func setup(t *testing.T)(*controller,string,string){
 t.Helper()
 store:=&auth.Store{File:filepath.Join(t.TempDir(),"tokens.json")}
 read,_,err:=store.Create("reader",[]string{auth.Read});if err!=nil{t.Fatal(err)}
 admin,_,err:=store.Create("admin",[]string{auth.Admin});if err!=nil{t.Fatal(err)}
 c:=&controller{Tokens:store}
 c.System=func()(systemInfo,error){return systemInfo{Hostname:"btv11",Architecture:"linux/arm64",MemoryTotal:1900000,MemoryAvailable:1000000,DiskTotal:14000000000,DiskAvailable:9000000000},nil}
 c.Containers=func(context.Context)([]containerInfo,error){return []containerInfo{{ID:"123456",Name:"mackna-btv-app-1",State:"running",Health:"healthy",Project:"mackna-btv",Service:"app"}},nil}
 c.HTTP=&http.Client{Transport:roundTripFunc(func(r *http.Request)(*http.Response,error){
  response:=`{"ok":true}`
  if strings.Contains(r.URL.Path,"/stats"){response=`{"cpuPercent":5.2}`}
  if strings.Contains(r.URL.Path,"/logs"){response=`{"lines":[{"stream":"stdout","text":"test"}]}`}
  return &http.Response{StatusCode:200,Body:io.NopCloser(strings.NewReader(response)),Header:http.Header{}},nil
 })}
 return c,read,admin
}
func req(c *controller,method,path,token string,basic bool)*httptest.ResponseRecorder{
 r:=httptest.NewRequest(method,path,nil)
 if token!=""{if basic{r.SetBasicAuth("admin",token)}else{r.Header.Set("Authorization","Bearer "+token)}}
 w:=httptest.NewRecorder();c.ServeHTTP(w,r)
 return w
}
func TestUnauthorizedAndMethods(t *testing.T){
 c,read,_:=setup(t)
 for _,path:=range []string{"/","/api/v1/system","/api/v1/containers","/api/v1/projects","/api/v1/containers/id/stats","/api/v1/containers/id/logs"}{
  if r:=req(c,"GET",path,"",false);r.Code!=401{t.Fatalf("%s: status %d",path,r.Code)}
 }
 if r:=req(c,"GET","/api/v1/containers/id/logs",read,false);r.Code!=401{t.Fatal("read token cannot fetch logs")}
 if r:=req(c,"POST","/api/v1/containers","",false);r.Code!=405{t.Fatal("non-read method allowed")}
 if r:=req(c,"GET","/api/v1/system","wrong",false);r.Code!=401{t.Fatal("bad credential allowed")}
}
func TestAPIAndDashboard(t *testing.T){
 c,read,admin:=setup(t)
 for _,path:=range []string{"/api/v1/system","/api/v1/containers","/api/v1/projects","/api/v1/containers/abc/stats"}{
  r:=req(c,"GET",path,read,false)
  if r.Code!=200 {t.Fatalf("%s status=%d body=%s",path,r.Code,r.Body.String())}
 }
 r:=req(c,"GET","/api/v1/containers/abc/logs?tail=10",admin,false)
 if r.Code!=200||!bytes.Contains(r.Body.Bytes(),[]byte("test")){t.Fatalf("logs: %d %s",r.Code,r.Body.String())}
 r=req(c,"GET","/",admin,true)
 if r.Code!=200||!strings.Contains(r.Body.String(),"MackNa Cloud")||!strings.Contains(r.Body.String(),"mackna-btv-app-1"){
  t.Fatalf("invalid dashboard response %d",r.Code)
 }
 if !strings.Contains(r.Header().Get("Content-Security-Policy"),"default-src 'none'"){t.Fatal("missing CSP")}
}
func TestHealthAndInputValidation(t *testing.T){
 c,_,admin:=setup(t)
 r:=req(c,"GET","/api/v1/health","",false)
 if r.Code!=200{t.Fatal("health must respond 200 with fake gate")}
 var status map[string]any
 if err:=json.Unmarshal(r.Body.Bytes(),&status);err!=nil||status["docker"]!=true{t.Fatal("bad health payload")}
 for _,path:=range []string{"/api/v1/containers/abc/logs?tail=1001","/api/v1/containers/abc/logs?tail=0","/api/v1/containers/..%2Fsecret"}{
  r:=req(c,"GET",path,admin,false)
  if r.Code<400{t.Fatalf("accepted invalid request %s",path)}
 }
 if r:=req(c,"GET","/api/v1/containers/abc/logs?tail=10",admin,false);r.Code!=200{t.Fatalf("good logs: %d",r.Code)}
}
func TestHumanBytes(t *testing.T){
 if humanBytes(1024)!="1.0 KiB"||humanBytes(1024*1024)!="1.0 MiB"{t.Fatal("byte formatter broken")}
}
