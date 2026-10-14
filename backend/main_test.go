package main

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func sampleAgent() *agent {
    return &agent{
        token: strings.Repeat("a", 64),
        system: func() (systemInfo,error) {
            return systemInfo{Hostname:"tvbox",Architecture:"linux/arm64",MemoryTotal:200,MemoryAvailable:100,DiskTotal:1000,DiskAvailable:900},nil
        },
        containers: func(context.Context) ([]containerInfo,error) {
            return []containerInfo{{ID:"abc",Name:"mackna-btv-db-1",Image:"postgres:16-alpine",State:"running",Status:"Up (healthy)"}},nil
        },
    }
}

func send(agent *agent, method, path, user, pass string) *httptest.ResponseRecorder {
    request := httptest.NewRequest(method,path,nil)
    if user != "" { request.SetBasicAuth(user,pass) }
    result := httptest.NewRecorder()
    agent.ServeHTTP(result,request)
    return result
}

func TestNoUnauthenticatedData(t *testing.T) {
    a := sampleAgent()
    for _, route := range []string{"/", "/api/v1/system","/api/v1/containers"} {
        res:=send(a, http.MethodGet,route,"","")
        if res.Code!=401 {t.Fatalf("%s returned %d, wanted 401",route,res.Code)}
        if strings.Contains(res.Body.String(),"tvbox") {t.Fatal("unauthenticated data leak")}
    }
    if res:=send(a,http.MethodGet,"/api/v1/system","admin","wrong");res.Code!=401 {t.Fatal("incorrect password accepted")}
    if res:=send(a,http.MethodGet,"/api/v1/system","root",a.token);res.Code!=401 {t.Fatal("incorrect user accepted")}
}

func TestHealthAndReadOnlyAPI(t *testing.T) {
    a:=sampleAgent()
    res:=send(a,http.MethodGet,"/api/v1/health","","")
    if res.Code!=200 {t.Fatalf("health returned %d",res.Code)}
    res=send(a,http.MethodGet,"/api/v1/system","admin",a.token)
    if res.Code!=200 {t.Fatalf("system returned %d: %s",res.Code,res.Body.String())}
    var sys systemInfo
    if err:=json.Unmarshal(res.Body.Bytes(),&sys);err!=nil||sys.Hostname!="tvbox" {t.Fatalf("invalid system response: %v %+v",err,sys)}
    res=send(a,http.MethodGet,"/api/v1/containers","admin",a.token)
    if res.Code!=200||!strings.Contains(res.Body.String(),"postgres:16-alpine") {t.Fatalf("container inventory failed: %d",res.Code)}
    res=send(a,http.MethodGet,"/","admin",a.token)
    if res.Code!=200||!strings.Contains(res.Body.String(),"MackNa Cloud")||!strings.Contains(res.Body.String(),"mackna-btv-db-1") {
        t.Fatalf("dashboard render failed: %d: %s",res.Code,res.Body.String())
    }
    if !strings.Contains(res.Header().Get("Content-Security-Policy"),"default-src 'none'") {t.Fatal("CSP missing")}
    res=send(a,http.MethodPost,"/api/v1/containers","admin",a.token)
    if res.Code!=405 {t.Fatalf("mutating HTTP method returned %d",res.Code)}
}

func TestHealthDoesNotRequireDocker(t *testing.T) {
    a:=sampleAgent()
    a.containers=func(context.Context)([]containerInfo,error){return nil,context.DeadlineExceeded}
    res:=send(a,http.MethodGet,"/api/v1/containers","admin",a.token)
    if res.Code!=503 {t.Fatalf("missing Docker should be 503, got %d",res.Code)}
    res=send(a,http.MethodGet,"/api/v1/health","","")
    if res.Code!=200 {t.Fatal("Docker must not break liveness")}
}

func TestHumanBytes(t *testing.T) {
    if humanBytes(1024)!="1.0 KiB" || humanBytes(1024*1024)!="1.0 MiB" {t.Fatal("unexpected memory formatting")}
}
