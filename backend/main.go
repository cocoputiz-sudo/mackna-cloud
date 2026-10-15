package main

import (
 "bufio"
 "context"
 "crypto/subtle"
 "embed"
 "encoding/json"
 "errors"
 "fmt"
 "html/template"
 "io"
 "log"
 "net"
 "net/http"
 "net/url"
 "os"
 "runtime"
 "strconv"
 "strings"
 "syscall"
 "time"

 "github.com/cocoputiz-sudo/mackna-cloud/backend/internal/audit"
 "github.com/cocoputiz-sudo/mackna-cloud/backend/internal/auth"
 "github.com/cocoputiz-sudo/mackna-cloud/backend/internal/gate"
)

//go:embed dashboard.html
var dashboardHTML embed.FS

const version = "0.3.0"
// Injected by CI at build time so deployed binaries can be identified reliably.
var buildCommit = "dev"
const listenAddress = "127.0.0.1:8090"
var page = template.Must(template.New("dashboard.html").Funcs(template.FuncMap{
 "human": humanBytes,
 "temperature": humanTemperature,
 "uptime": humanUptime,
}).ParseFS(dashboardHTML,"dashboard.html"))

func humanTemperature(value *float64) string {
 if value == nil { return "n/d" }
 return fmt.Sprintf("%.1f °C", *value)
}

func humanUptime(seconds float64) string {
 duration := int64(seconds)
 if duration < 0 { duration = 0 }
 days := duration / 86400
 hours := (duration % 86400) / 3600
 minutes := (duration % 3600) / 60
 if days > 0 { return fmt.Sprintf("%dd %dh %dmin",days,hours,minutes) }
 if hours > 0 { return fmt.Sprintf("%dh %dmin",hours,minutes) }
 if minutes > 0 { return fmt.Sprintf("%dmin %ds",minutes,duration%60) }
 return fmt.Sprintf("%ds",duration)
}

func humanBytes(n uint64)string{
 const k=1024
 if n<k{return fmt.Sprintf("%d B",n)}
 v:=float64(n)
 for _,unit:=range []string{"KiB","MiB","GiB","TiB"}{
  v/=k
  if v<k{return fmt.Sprintf("%.1f %s",v,unit)}
 }
 return fmt.Sprintf("%.1f PiB",v/k)
}

type systemInfo struct{
 Hostname string `json:"hostname"`
 Architecture string `json:"architecture"`
 UptimeSeconds float64 `json:"uptime_seconds"`
 LoadAverage1m float64 `json:"load_average_1m"`
 MemoryTotal uint64 `json:"memory_total_bytes"`
 MemoryAvailable uint64 `json:"memory_available_bytes"`
 DiskTotal uint64 `json:"disk_total_bytes"`
 DiskAvailable uint64 `json:"disk_available_bytes"`
 TemperatureC *float64 `json:"temperature_c,omitempty"`
}
type containerInfo = gate.Container

func readSystem()(systemInfo,error){
 info:=systemInfo{Architecture:"linux/"+runtime.GOARCH}
 var err error
 info.Hostname,err=os.Hostname();if err!=nil{return info,err}
 b,err:=os.ReadFile("/proc/uptime");if err!=nil{return info,err}
 fields:=strings.Fields(string(b));if len(fields)<1{return info,errors.New("malformed uptime")}
 info.UptimeSeconds,err=strconv.ParseFloat(fields[0],64);if err!=nil{return info,err}
 b,err=os.ReadFile("/proc/loadavg");if err!=nil{return info,err}
 fields=strings.Fields(string(b));if len(fields)<1{return info,errors.New("malformed loadavg")}
 info.LoadAverage1m,err=strconv.ParseFloat(fields[0],64);if err!=nil{return info,err}
 f,err:=os.Open("/proc/meminfo");if err!=nil{return info,err}
 defer f.Close()
 var total,avail bool
 sc:=bufio.NewScanner(f)
 for sc.Scan(){
  fields:=strings.Fields(sc.Text());if len(fields)<2{continue}
  if fields[0]!="MemTotal:"&&fields[0]!="MemAvailable:"{continue}
  n,e:=strconv.ParseUint(fields[1],10,64);if e!=nil{return info,e}
  if fields[0]=="MemTotal:"{info.MemoryTotal=n*1024;total=true}
  if fields[0]=="MemAvailable:"{info.MemoryAvailable=n*1024;avail=true}
 }
 if err:=sc.Err();err!=nil{return info,err}
 if !total||!avail{return info,errors.New("meminfo metrics missing")}
 var disk syscall.Statfs_t
 if err=syscall.Statfs("/",&disk);err!=nil{return info,err}
 info.DiskTotal=disk.Blocks*uint64(disk.Bsize)
 info.DiskAvailable=disk.Bavail*uint64(disk.Bsize)
 for i:=0;i<20;i++{
  p:=fmt.Sprintf("/sys/class/thermal/thermal_zone%d/temp",i)
  b,e:=os.ReadFile(p);if e!=nil{continue}
  milli,e:=strconv.ParseFloat(strings.TrimSpace(string(b)),64)
  if e!=nil||milli<=0||milli>150000{continue}
  c:=milli/1000
  if info.TemperatureC==nil||c>*info.TemperatureC{info.TemperatureC=&c}
 }
 return info,nil
}

type controller struct {
 Audit *audit.Log
 Tokens *auth.Store
 HTTP *http.Client
 System func()(systemInfo,error)
 Containers func(context.Context)([]containerInfo,error)
}

func gateClient(socket string)*http.Client{
 transport:=&http.Transport{
  DialContext:func(ctx context.Context,_,_ string)(net.Conn,error){
   return (&net.Dialer{Timeout:3*time.Second}).DialContext(ctx,"unix",socket)
  },
  MaxIdleConns:4,ResponseHeaderTimeout:10*time.Second,
 }
 return &http.Client{Transport:transport,Timeout:15*time.Second,CheckRedirect:func(*http.Request,[]*http.Request)error{return http.ErrUseLastResponse}}
}
func newController(tokenStore *auth.Store, socket string)*controller{
 c:=&controller{Tokens:tokenStore,HTTP:gateClient(socket),System:readSystem}
 c.Containers=c.listContainers
 return c
}
func (c *controller) call(ctx context.Context, path string)([]byte,int,error){
 req,err:=http.NewRequestWithContext(ctx,"GET","http://gate"+path,nil)
 if err!=nil{return nil,0,err}
 res,err:=c.HTTP.Do(req);if err!=nil{return nil,0,err}
 defer res.Body.Close()
 // Upper bound on all read responses; see gate for smaller per-method limits.
 var b strings.Builder
 if _,err=ioCopyMax(&b,res.Body,4<<20);err!=nil{return nil,res.StatusCode,err}
 return []byte(b.String()),res.StatusCode,nil
}
func ioCopyMax(dst *strings.Builder,src io.Reader,max int64)(int64,error){
 n,err:=io.Copy(dst,io.LimitReader(src,max+1))
 if err!=nil{return n,err}
 if n>max{return n,errors.New("response too large")}
 return n,nil
}
func (c *controller) listContainers(ctx context.Context)([]containerInfo,error){
 b,status,err:=c.call(ctx,"/containers/json?all=1")
 if err!=nil{return nil,err}
 if status!=200{return nil,errors.New("gate unavailable")}
 var records []containerInfo
 if err=json.Unmarshal(b,&records);err!=nil{return nil,err}
 return records,nil
}
func jsonResponse(w http.ResponseWriter,status int,v any){
 w.Header().Set("Content-Type","application/json; charset=utf-8")
 w.WriteHeader(status)
 _=json.NewEncoder(w).Encode(v)
}
func (c *controller) authorized(r *http.Request,scope string)(auth.Token,bool){
 hdr:=r.Header.Get("Authorization")
 var presented string
 if strings.HasPrefix(hdr,"Bearer "){presented=strings.TrimSpace(strings.TrimPrefix(hdr,"Bearer "))}
 if u,p,ok:=r.BasicAuth();ok&&subtle.ConstantTimeCompare([]byte(u),[]byte("admin"))==1{presented=p}
 tok,ok:=c.Tokens.Authenticate(presented)
 return tok,ok&&tok.Has(scope)
}
func secureHeaders(w http.ResponseWriter){
 h:=w.Header()
 h.Set("Cache-Control","no-store")
 h.Set("X-Content-Type-Options","nosniff")
 h.Set("X-Frame-Options","DENY")
 h.Set("Referrer-Policy","no-referrer")
 h.Set("Content-Security-Policy","default-src 'none'; script-src 'self'; connect-src 'self'; style-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'")
}
func (c *controller) logAudit(r *http.Request,tokenID,action string,status int){
 if c.Audit==nil{return}
 if err:=c.Audit.Write(audit.Event{Action:action,TokenID:tokenID,Path:r.URL.Path,Status:status});err!=nil{log.Printf("audit trail failed: %v",err)}
}
func (c *controller) ServeHTTP(w http.ResponseWriter,r *http.Request){
 secureHeaders(w)
 if r.Method!="GET"{w.Header().Set("Allow","GET");jsonResponse(w,405,map[string]string{"error":"method_not_allowed"});return}
 if r.URL.Path=="/api/v1/health"{
  ctx,cancel:=context.WithTimeout(r.Context(),3*time.Second);defer cancel()
  _,status,err:=c.call(ctx,"/_ping")
  healthy:=err==nil&&status==200
  if healthy{jsonResponse(w,200,map[string]any{"ok":true,"docker":true,"version":version,"build":buildCommit})}else{jsonResponse(w,503,map[string]any{"ok":false,"docker":false,"version":version,"build":buildCommit})}
  return
 }
 scope:=auth.Read
 if strings.HasSuffix(r.URL.Path,"/logs"){scope=auth.Logs}
 if r.URL.Path=="/api/v1/audit"{scope=auth.Admin}
 tok,ok:=c.authorized(r,scope)
 if !ok{
  if r.URL.Path=="/" || r.URL.Path=="/projects" || r.URL.Path=="/projects/view" || r.URL.Path=="/services/view" || r.URL.Path=="/postgres/lab" || r.URL.Path=="/postgres/lab.js" {w.Header().Set("WWW-Authenticate",`Basic realm="MackNa Cloud", charset="UTF-8"`)}
  jsonResponse(w,401,map[string]string{"error":"unauthorized"})
  log.Printf("AUDIT %s",auditLine("auth_denied",r,"",401))
  if strings.HasPrefix(r.URL.Path,"/api/v1/postgres/lab/")||r.URL.Path=="/api/v1/audit"{c.logAudit(r,"","auth_denied",401)}
  return
 }
 log.Printf("AUDIT %s",auditLine("api_access",r,tok.ID,200))
 switch r.URL.Path{
 case "/api/v1/audit":
  if c.Audit==nil{jsonResponse(w,503,map[string]string{"error":"audit_unavailable"});return}
  if !queryOnly(r.URL.Query(),"limit"){jsonResponse(w,400,map[string]string{"error":"invalid_query"});return}
  limit:=50
  if r.URL.Query().Has("limit"){var err error;limit,err=strconv.Atoi(r.URL.Query().Get("limit"));if err!=nil||limit<1||limit>100{jsonResponse(w,400,map[string]string{"error":"invalid_limit"});return}}
  entries,err:=c.Audit.Tail(limit)
  if err!=nil{jsonResponse(w,503,map[string]string{"error":"audit_unavailable"});return}
  jsonResponse(w,200,map[string]any{"entries":entries})
 case "/postgres/lab":
  c.servePostgresLabPage(w,r)
 case "/postgres/lab.js":
  c.servePostgresLabScript(w,r)
 case "/projects", "/projects/view", "/services/view":
  c.serveConsole(w,r,tok)
 case "/api/v1/system","/api/v1/node":
  v,err:=c.System();if err!=nil{jsonResponse(w,503,map[string]string{"error":"metrics_unavailable"});return}
  jsonResponse(w,200,v)
 case "/api/v1/containers","/api/v1/projects":
  list,err:=c.Containers(r.Context());if err!=nil{jsonResponse(w,503,map[string]string{"error":"docker_unavailable"});return}
  if r.URL.Path=="/api/v1/projects"{
   projectMap:=map[string][]containerInfo{}
   for _,item:=range list{p:=item.Project;if p==""{p="(standalone)"};projectMap[p]=append(projectMap[p],item)}
   jsonResponse(w,200,map[string]any{"projects":projectMap})
  }else{jsonResponse(w,200,list)}
 case "/":
  info,errSys:=c.System()
  containers,errDock:=c.Containers(r.Context())
  w.Header().Set("Content-Type","text/html; charset=utf-8")
  if err:=page.Execute(w,struct{
   Node string;Info systemInfo;SystemOK bool;DockerOK bool;Containers []containerInfo;Version string
  }{"Node 01",info,errSys==nil,errDock==nil,containers,version});err!=nil{log.Printf("render error: %v",err)}
 default:
  if strings.HasPrefix(r.URL.Path,"/api/v1/postgres/lab/") {c.servePostgresLab(w,r);return}
  const prefix="/api/v1/containers/"
  if !strings.HasPrefix(r.URL.Path,prefix){jsonResponse(w,404,map[string]string{"error":"not_found"});return}
  sub:=strings.TrimPrefix(r.URL.Path,prefix)
  parts:=strings.Split(sub,"/")
  if len(parts)<1||len(parts)>2||!validContainerName(parts[0]){
   jsonResponse(w,400,map[string]string{"error":"invalid_container_id"});return
  }
  target:="/containers/"+url.PathEscape(parts[0])+"/json"
  if len(parts)==2 {
   switch parts[1]{
   case "stats":target="/containers/"+url.PathEscape(parts[0])+"/stats?stream=false"
   case "logs":
    tail:=r.URL.Query().Get("tail");if tail==""{tail="200"}
    n,err:=strconv.Atoi(tail);if err!=nil||n<1||n>gate.MaxLogTail{jsonResponse(w,400,map[string]string{"error":"invalid_tail"});return}
    q:=url.Values{"tail":{strconv.Itoa(n)}}
    if since:=r.URL.Query().Get("since");since!=""{
     if _,err:=strconv.ParseInt(since,10,64);err!=nil{jsonResponse(w,400,map[string]string{"error":"invalid_since"});return}
     q.Set("since",since)
    }
    target="/containers/"+url.PathEscape(parts[0])+"/logs?"+q.Encode()
   default:jsonResponse(w,404,map[string]string{"error":"not_found"});return
   }
  }else if len(r.URL.Query())>0{jsonResponse(w,400,map[string]string{"error":"unexpected_query"});return}
  b,status,err:=c.call(r.Context(),target)
  if err!=nil{jsonResponse(w,502,map[string]string{"error":"gate_unavailable"});return}
  if status!=200{w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_,_=w.Write(b);return}
  w.Header().Set("Content-Type","application/json")
  w.WriteHeader(200);_,_=w.Write(b)
 }
}
func validContainerName(v string)bool{
 if len(v)<1||len(v)>128{return false}
 for i,c:=range v{
  if c>='a'&&c<='z'||c>='A'&&c<='Z'||c>='0'&&c<='9'{continue}
  if i>0&&(c=='.'||c=='_'||c=='-'){continue}
  return false
 }
 return true
}
func auditLine(event string,r *http.Request,id string,code int)string{
 b,_:=json.Marshal(map[string]any{"event":event,"tokenId":id,"method":r.Method,"path":r.URL.Path,"status":code,"at":time.Now().UTC().Format(time.RFC3339Nano)})
 return string(b)
}
func usage(){fmt.Fprintln(os.Stderr,"usage: mackna-agent serve | token create NAME SCOPES | token list | token revoke ID")}
func main(){
 data:=os.Getenv("MACKNA_DATA_DIR");if data==""{data="/var/lib/mackna"}
 tokenStore:=&auth.Store{File:data+"/tokens.json"}
 if len(os.Args)<2{usage();os.Exit(2)}
 switch os.Args[1] {
 case "token":
  if len(os.Args)<3{usage();os.Exit(2)}
  switch os.Args[2]{
  case "create":
   if len(os.Args)!=5{usage();os.Exit(2)}
   plain,tok,err:=tokenStore.Create(os.Args[3],strings.Split(os.Args[4],","))
   if err!=nil{log.Fatal(err)}
   fmt.Printf("Token %s criado; guarde o segredo agora (não será exibido novamente):\n%s\n",tok.ID,plain)
  case "list":
   list,err:=tokenStore.List();if err!=nil{log.Fatal(err)}
   for _,tok:=range list{fmt.Printf("%s %s scopes=%s revoked=%t\n",tok.ID,tok.Name,strings.Join(tok.Scopes,","),tok.Revoked)}
  case "revoke":
   if len(os.Args)!=4{usage();os.Exit(2)}
   if err:=tokenStore.Revoke(os.Args[3]);err!=nil{log.Fatal(err)}
   fmt.Println("Token revogado")
  default:usage();os.Exit(2)
  }
 case "serve":
  socket:=os.Getenv("MACKNA_GATE_SOCKET")
  if socket==""{socket="/run/mackna-gate/docker-ro.sock"}
  ctrl:=newController(tokenStore,socket)
  if trail,err:=audit.Open(data+"/audit.jsonl",2<<20);err!=nil{log.Printf("audit log not available: %v",err)}else{ctrl.Audit=trail;defer trail.Close()}
  server:=&http.Server{
   Addr:listenAddress,Handler:ctrl,
   ReadHeaderTimeout:5*time.Second,WriteTimeout:20*time.Second,IdleTimeout:30*time.Second,MaxHeaderBytes:8192,
  }
  log.Printf("MackNa controller v%s on %s (SSH tunnel only)",version,listenAddress)
  if err:=server.ListenAndServe();err!=nil&& !errors.Is(err,http.ErrServerClosed){log.Fatal(err)}
 default:usage();os.Exit(2)
 }
}
