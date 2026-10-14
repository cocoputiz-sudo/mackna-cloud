package main

import (
 "context"
 "embed"
 "encoding/json"
 "html/template"
 "net/http"
 "net/url"
 "sort"
 "strconv"
 "strings"
 "time"

 "github.com/cocoputiz-sudo/mackna-cloud/backend/internal/auth"
 "github.com/cocoputiz-sudo/mackna-cloud/backend/internal/gate"
)

//go:embed console.html
var consoleFS embed.FS

var consolePage = template.Must(template.New("console.html").
 Funcs(template.FuncMap{"human":humanBytes}).
 ParseFS(consoleFS,"console.html"))

type projectSummary struct {
 Name string
 Services []containerInfo
 Total int
 Running int
 Degraded int
}

type inspectedService struct {
 ID string
 Name string
 Image string
 State string
 Running bool
 Health string
 OOMKilled bool
 RestartCount int
 MemoryLimitBytes uint64
 Project string
 Service string
}
type serviceMetrics struct {
 CPUPercent float64
 MemoryUsageBytes uint64
 MemoryLimitBytes uint64
}
type serviceLogs struct {
 Lines []gate.LogLine
 Truncated bool
}
type consoleData struct {
 Section string
 Title string
 Version string
 Generated string
 Projects []projectSummary
 Project projectSummary
 Service containerInfo
 Inspection inspectedService
 InspectOK bool
 Metrics serviceMetrics
 MetricsOK bool
 CanLogs bool
 ShowLogs bool
 LogLines []gate.LogLine
 LogError string
 Tail int
 Error string
 Total int
}

// Compose project labels come from the gate's fixed allowlist, not arbitrary
// Docker labels or a user-writable database.
func projectName(item containerInfo)string{
 if item.Project=="" {return "(standalone)"}
 return item.Project
}
func summarizeProjects(items []containerInfo)[]projectSummary{
 grouped:=map[string]*projectSummary{}
 for _,item:=range items{
  name:=projectName(item)
  p:=grouped[name]
  if p==nil {p=&projectSummary{Name:name};grouped[name]=p}
  p.Services=append(p.Services,item)
  p.Total++
  if item.State=="running"{p.Running++}
  if item.State!="running" || item.Health=="unhealthy"{p.Degraded++}
 }
 projects:=make([]projectSummary,0,len(grouped))
 for _,p:=range grouped{
  sort.Slice(p.Services,func(i,j int)bool{return p.Services[i].Name<p.Services[j].Name})
  projects=append(projects,*p)
 }
 sort.Slice(projects,func(i,j int)bool{return projects[i].Name<projects[j].Name})
 return projects
}
func queryOnly(q url.Values, required string, allowed ...string)bool{
 safe:=map[string]bool{required:true}
 for _,a:=range allowed{safe[a]=true}
 if len(q[required])!=1 || strings.TrimSpace(q.Get(required))==""{return false}
 for key,values:=range q {
  if !safe[key]||len(values)!=1{return false}
 }
 return true
}
func (c *controller) readJSON(ctx context.Context,target string,dst any)bool{
 b,code,err:=c.call(ctx,target)
 return err==nil&&code==http.StatusOK&&json.Unmarshal(b,dst)==nil
}
func (c *controller) serveConsole(w http.ResponseWriter,r *http.Request,tok auth.Token){
 data:=consoleData{Section:"projects",Title:"Projetos",Version:version,Generated:time.Now().Format("02/01/2006 15:04:05"),CanLogs:tok.Has(auth.Logs),Tail:100}
 switch r.URL.Path{
 case "/projects":
  if len(r.URL.Query())!=0{http.Error(w,"unexpected_query",400);return}
 case "/projects/view":
  if !queryOnly(r.URL.Query(),"name"){http.Error(w,"invalid_project",400);return}
  data.Section="project";data.Title="Serviços do projeto"
 case "/services/view":
  q:=r.URL.Query()
  if !queryOnly(q,"id","logs","tail"){http.Error(w,"invalid_service",400);return}
  if q.Has("logs")&&q.Get("logs")!="1"{http.Error(w,"invalid_logs",400);return}
  if q.Has("tail")&&!q.Has("logs"){http.Error(w,"tail_requires_logs",400);return}
  if q.Has("logs") {
   if !data.CanLogs{http.Error(w,"logs_scope_required",403);return}
   data.ShowLogs=true
   if q.Has("tail"){
    tail,err:=strconv.Atoi(q.Get("tail"))
    if err!=nil||tail<1||tail>gate.MaxLogTail{http.Error(w,"invalid_tail",400);return}
    data.Tail=tail
   }
  }
  data.Section="service";data.Title="Detalhes do serviço"
 default:
  http.NotFound(w,r);return
 }
 ctx,cancel:=context.WithTimeout(r.Context(),14*time.Second)
 defer cancel()
 containers,err:=c.Containers(ctx)
 if err!=nil {
  data.Error="Não foi possível consultar o inventário do Docker."
  w.Header().Set("Content-Type","text/html; charset=utf-8")
  w.WriteHeader(http.StatusServiceUnavailable)
  renderConsole(w,data)
  return
 }
 data.Total=len(containers)
 data.Projects=summarizeProjects(containers)
 if data.Section=="project"{
  found:=false
  for _,p:=range data.Projects{
   if p.Name==r.URL.Query().Get("name"){data.Project=p;found=true;break}
  }
  if !found{http.NotFound(w,r);return}
 }
 if data.Section=="service"{
  selected:=false
  for _,item:=range containers{
   if item.ID==r.URL.Query().Get("id"){data.Service=item;selected=true;break}
  }
  if !selected{http.NotFound(w,r);return}
  id:=url.PathEscape(data.Service.ID)
  data.InspectOK=c.readJSON(ctx,"/containers/"+id+"/json",&data.Inspection)
  data.MetricsOK=c.readJSON(ctx,"/containers/"+id+"/stats?stream=false",&data.Metrics)
  if data.ShowLogs {
   var lines serviceLogs
   if c.readJSON(ctx,"/containers/"+id+"/logs?tail="+strconv.Itoa(data.Tail),&lines){
    data.LogLines=lines.Lines
   }else{
    data.LogError="Logs indisponíveis. Confira o estado do serviço."
   }
  }
 }
 renderConsole(w,data)
}
func renderConsole(w http.ResponseWriter,data consoleData){
 w.Header().Set("Content-Type","text/html; charset=utf-8")
 if err:=consolePage.Execute(w,data);err!=nil {
  // Request path and credentials are never written to error logs.
  // An invalid template is a release-blocking failure, checked in tests.
  return
 }
}
