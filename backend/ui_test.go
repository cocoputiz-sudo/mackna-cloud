package main

import (
 "context"
 "errors"
 "net/http"
 "strings"
 "testing"

 "github.com/cocoputiz-sudo/mackna-cloud/backend/internal/auth"
)

func TestConsoleProjectsAndServices(t *testing.T){
 c,read,admin:=setup(t)
 for _,path:=range []string{"/projects","/projects/view?name=mackna-btv","/services/view?id=123456"}{
  got:=req(c,"GET",path,read,true)
  if got.Code!=http.StatusOK{t.Fatalf("%s status %d: %s",path,got.Code,got.Body.String())}
  if !strings.Contains(got.Body.String(),"mackna-btv"){t.Fatalf("%s absent project",path)}
 }
 index:=req(c,"GET","/projects",read,true)
 if !strings.Contains(index.Body.String(),"1 / 1"){t.Fatal("project summary missing")}
 detail:=req(c,"GET","/services/view?id=123456",admin,true)
 if !strings.Contains(detail.Body.String(),"5.2%") {t.Fatal("CPU missing")}
 if !strings.Contains(detail.Body.String(),"Consultar logs"){t.Fatal("log UI missing")}
 if strings.Contains(detail.Body.String(),"[stdout] test"){t.Fatal("logs loaded without explicit request")}
 logs:=req(c,"GET","/services/view?id=123456&logs=1&tail=50",admin,true)
 if logs.Code!=200||!strings.Contains(logs.Body.String(),"[stdout] test"){t.Fatalf("logs: %d, %s",logs.Code,logs.Body.String())}
}

func TestConsolePermissionsAndInput(t *testing.T){
 c,read,_:=setup(t)
 for _,path:=range []string{"/projects","/projects/view?name=mackna-btv","/services/view?id=123456"}{
  if got:=req(c,"GET",path,"",false);got.Code!=401{t.Fatalf("auth for %s: %d",path,got.Code)}
 }
 if got:=req(c,"GET","/services/view?id=123456&logs=1",read,false);got.Code!=403{t.Fatalf("read token requested logs: %d",got.Code)}
 for _,path:=range []string{
  "/services/view?id=123456&logs=1&tail=1001",
  "/services/view?id=123456&logs=1&tail=-1",
  "/services/view?id=123456&tail=50",
  "/services/view?id=123456&foo=bar",
  "/services/view?id=123456&id=bad",
  "/services/view?id=../../etc/passwd",
  "/projects/view?name=a&name=b",
  "/projects?delete=true",
 }{
  got:=req(c,"GET",path,read,false)
  if got.Code<400{t.Fatalf("unexpected success for %s: %d",path,got.Code)}
 }
 if got:=req(c,"POST","/services/view?id=123456",read,false);got.Code!=405{t.Fatal("mutations forbidden")}
 if got:=req(c,"GET","/services/view?id=unknown",read,false);got.Code!=404{t.Fatal("unknown service must 404")}
}

func TestConsoleEscapesDockerNamesAndLogs(t *testing.T){
 c,_,admin:=setup(t)
 c.Containers=func(context.Context)([]containerInfo,error){
  return []containerInfo{{ID:"123456",Name:"<script>alert(1)</script>",Project:"p<script>",State:"running",Health:"healthy"}},nil
 }
 for _,path:=range []string{"/projects","/projects/view?name=p%3Cscript%3E","/services/view?id=123456&logs=1"}{
  got:=req(c,"GET",path,admin,false)
  if got.Code!=200{t.Fatalf("%s: %d",path,got.Code)}
  if strings.Contains(got.Body.String(),"<script>"){t.Fatal("untrusted Docker text interpreted as HTML")}
 }
}
func TestConsoleGracefulInventoryError(t *testing.T){
 c,read,_:=setup(t)
 c.Containers=func(context.Context)([]containerInfo,error){return nil,errors.New("socket offline")}
 got:=req(c,"GET","/projects",read,false)
 if got.Code!=503||!strings.Contains(got.Body.String(),"inventário"){t.Fatalf("bad error display: %d %s",got.Code,got.Body.String())}
}
func TestSummarizeProjects(t *testing.T){
 got:=summarizeProjects([]containerInfo{
  {Name:"healthy",Project:"app",State:"running",Health:"healthy"},
  {Name:"unhealthy",Project:"app",State:"running",Health:"unhealthy"},
  {Name:"standalone",State:"exited",Health:"none"},
 })
 if len(got)!=2 {t.Fatalf("want 2 projects, got %d",len(got))}
 if got[1].Name!="app" || got[1].Degraded!=1 || got[1].Running!=2 {t.Fatalf("incorrect status: %+v",got[1])}
 if got[0].Name!="(standalone)"||got[0].Degraded!=1{t.Fatalf("incorrect standalone: %+v",got[0])}
}
func TestExplicitLogsScope(t *testing.T){
 c,_,_:=setup(t)
 token,_,err:=c.Tokens.Create("log-reader",[]string{auth.Read,auth.Logs})
 if err!=nil{t.Fatal(err)}
 got:=req(c,"GET","/services/view?id=123456&logs=1&tail=100",token,false)
 if got.Code!=200||!strings.Contains(got.Body.String(),"[stdout] test"){t.Fatal("read+logs token should view logs")}
}
