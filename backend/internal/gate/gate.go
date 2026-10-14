// Package gate is a narrowly-scoped Docker API read-only proxy.
// The control plane never needs direct access to /var/run/docker.sock.
package gate

import (
 "context"
 "encoding/binary"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net"
 "net/http"
 "net/url"
 "regexp"
 "strconv"
 "strings"
 "time"
)

const MaxLogTail = 1000
const maxBody = 4 << 20
var idRx = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

func DockerClient(socket string) *http.Client {
 transport := &http.Transport{
  DialContext: func(ctx context.Context, _, _ string) (net.Conn,error) {
   return (&net.Dialer{Timeout:3*time.Second}).DialContext(ctx,"unix",socket)
  },
  MaxIdleConns: 4,
  ResponseHeaderTimeout: 8*time.Second,
 }
 return &http.Client{Transport:transport,Timeout:15*time.Second,CheckRedirect:func(*http.Request,[]*http.Request)error{return http.ErrUseLastResponse}}
}

type Service struct { Docker *http.Client }

func jsonOut(w http.ResponseWriter, code int, v any) {
 w.Header().Set("Content-Type","application/json")
 w.Header().Set("Cache-Control","no-store")
 w.WriteHeader(code)
 _ = json.NewEncoder(w).Encode(v)
}

func deny(w http.ResponseWriter, reason string) { jsonOut(w, http.StatusForbidden,map[string]string{"error":reason}) }

func (s Service) fetch(ctx context.Context, path string, q url.Values, limit int64) ([]byte,int,error) {
 u:="http://docker"+path
 if len(q)>0 {u+="?"+q.Encode()}
 req,err:=http.NewRequestWithContext(ctx,"GET",u,nil)
 if err!=nil {return nil,0,err}
 res,err:=s.Docker.Do(req)
 if err!=nil{return nil,0,err}
 defer res.Body.Close()
 b,err:=io.ReadAll(io.LimitReader(res.Body,limit+1))
 if err!=nil{return nil,res.StatusCode,err}
 if int64(len(b))>limit {return nil,res.StatusCode,errors.New("upstream response exceeded size limit")}
 return b,res.StatusCode,nil
}

type Container struct {
 ID string `json:"id"`
 Name string `json:"name"`
 Image string `json:"image"`
 State string `json:"state"`
 Status string `json:"status"`
 Health string `json:"health"`
 Project string `json:"project,omitempty"`
 Service string `json:"service,omitempty"`
}
func healthStatus(s string)string {
 if strings.Contains(s,"(unhealthy)"){return "unhealthy"}
 if strings.Contains(s,"(healthy)"){return "healthy"}
 if strings.Contains(s,"(health: starting)"){return "starting"}
 return "none"
}
func shortID(id string)string {if len(id)>12{return id[:12]};return id}

func (s Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
 if r.Method!="GET" {deny(w,"method_not_allowed");return}
 path:=r.URL.Path
 q:=r.URL.Query()
 if len(q)>4 {deny(w,"invalid_query");return}
 if path=="/_ping" || path=="/version" {
  if len(q)>0 {deny(w,"unexpected_query");return}
  b,status,err:=s.fetch(r.Context(),path,nil,2048)
  if err!=nil {jsonOut(w,502,map[string]string{"error":"docker_unavailable"});return}
  if status!=200 {jsonOut(w,502,map[string]string{"error":"docker_unavailable"});return}
  if path=="/_ping" {jsonOut(w,200,map[string]bool{"ok":true})} else {
   var d struct {Version string `json:"Version"`; Arch string `json:"Arch"`; Os string `json:"Os"`}
   if json.Unmarshal(b,&d)!=nil {jsonOut(w,502,map[string]string{"error":"invalid_docker_data"});return}
   jsonOut(w,200,map[string]string{"version":d.Version,"architecture":d.Arch,"os":d.Os})
  }
  return
 }
 if path=="/containers/json" {
  if len(q)>1 || (q.Get("all")!="" && q.Get("all")!="1") {deny(w,"invalid_query");return}
  b,status,err:=s.fetch(r.Context(),path,url.Values{"all":{"1"}},maxBody)
  if err!=nil||status!=200 {jsonOut(w,502,map[string]string{"error":"docker_unavailable"});return}
  var raw []struct {
   ID string `json:"Id"`
   Names []string `json:"Names"`
   Image string `json:"Image"`
   State string `json:"State"`
   Status string `json:"Status"`
   Labels map[string]string `json:"Labels"`
  }
  if json.Unmarshal(b,&raw)!=nil {jsonOut(w,502,map[string]string{"error":"invalid_docker_data"});return}
  out:=make([]Container,0,len(raw))
  for _,c:=range raw {
   name:="";if len(c.Names)>0 {name=strings.TrimPrefix(c.Names[0],"/")}
   out=append(out,Container{ID:shortID(c.ID),Name:name,Image:c.Image,State:c.State,Status:c.Status,
    Health:healthStatus(c.Status),Project:c.Labels["com.docker.compose.project"],Service:c.Labels["com.docker.compose.service"]})
  }
  jsonOut(w,200,out);return
 }
 parts:=strings.Split(strings.Trim(path,"/"),"/")
 if len(parts)!=3 || parts[0]!="containers" || !idRx.MatchString(parts[1]) {
  deny(w,"path_not_allowed");return
 }
 id,kind:=parts[1],parts[2]
 switch kind {
 case "json":
  if len(q)!=0{deny(w,"unexpected_query");return}
  b,status,err:=s.fetch(r.Context(),"/containers/"+url.PathEscape(id)+"/json",nil,maxBody)
  if status==404 {jsonOut(w,404,map[string]string{"error":"not_found"});return}
  if err!=nil||status!=200 {jsonOut(w,502,map[string]string{"error":"docker_unavailable"});return}
  var raw struct {
   ID string `json:"Id"`; Name string `json:"Name"`; RestartCount int `json:"RestartCount"`
   State struct {Status string `json:"Status"`; Running bool `json:"Running"`; OOMKilled bool `json:"OOMKilled"`; Health *struct{Status string `json:"Status"`} `json:"Health"`} `json:"State"`
   HostConfig struct{Memory int64 `json:"Memory"`} `json:"HostConfig"`
   Config struct {Image string `json:"Image"`; Labels map[string]string `json:"Labels"`} `json:"Config"`
  }
  if json.Unmarshal(b,&raw)!=nil {jsonOut(w,502,map[string]string{"error":"invalid_docker_data"});return}
  health:="none";if raw.State.Health!=nil{health=raw.State.Health.Status}
  // Deliberately do not return Docker Config.Env, Cmd, Entrypoint, mounts or arbitrary labels.
  jsonOut(w,200,map[string]any{
   "id":shortID(raw.ID),"name":strings.TrimPrefix(raw.Name,"/"),"image":raw.Config.Image,
   "state":raw.State.Status,"running":raw.State.Running,"health":health,
   "oomKilled":raw.State.OOMKilled,"restartCount":raw.RestartCount,
   "memoryLimitBytes":raw.HostConfig.Memory,
   "project":raw.Config.Labels["com.docker.compose.project"],
   "service":raw.Config.Labels["com.docker.compose.service"],
  })
 case "stats":
  if len(q)!=1||q.Get("stream")!="false"{deny(w,"stats_stream_forbidden");return}
  b,status,err:=s.fetch(r.Context(),"/containers/"+url.PathEscape(id)+"/stats",url.Values{"stream":{"false"}},maxBody)
  if status==404 {jsonOut(w,404,map[string]string{"error":"not_found"});return}
  if err!=nil||status!=200 {jsonOut(w,502,map[string]string{"error":"docker_unavailable"});return}
  var raw struct{
   CPU struct{Usage struct{Total uint64 `json:"total_usage"`}`json:"cpu_usage"`; System uint64 `json:"system_cpu_usage"`; Online uint64 `json:"online_cpus"`}`json:"cpu_stats"`
   PreCPU struct{Usage struct{Total uint64 `json:"total_usage"`}`json:"cpu_usage"`; System uint64 `json:"system_cpu_usage"`}`json:"precpu_stats"`
   Mem struct{Usage uint64 `json:"usage"`; Limit uint64 `json:"limit"`; Stats map[string]uint64 `json:"stats"`}`json:"memory_stats"`
  }
  if json.Unmarshal(b,&raw)!=nil {jsonOut(w,502,map[string]string{"error":"invalid_docker_data"});return}
  used:=raw.Mem.Usage
  cache:=raw.Mem.Stats["inactive_file"]
  if used>=cache{used-=cache}
  var cpu float64
  if raw.CPU.System>raw.PreCPU.System&&raw.CPU.Usage.Total>=raw.PreCPU.Usage.Total {
   cpu=float64(raw.CPU.Usage.Total-raw.PreCPU.Usage.Total)/float64(raw.CPU.System-raw.PreCPU.System)*float64(raw.CPU.Online)*100
  }
  jsonOut(w,200,map[string]any{"cpuPercent":cpu,"memoryUsageBytes":used,"memoryLimitBytes":raw.Mem.Limit})
 case "logs":
  if len(q)>2 {deny(w,"invalid_query");return}
  if q.Has("follow") || q.Has("stream") || q.Has("stdout") || q.Has("stderr") || q.Has("timestamps") {deny(w,"logs_follow_or_override_forbidden");return}
  for key:=range q {if key!="tail" && key!="since" {deny(w,"unknown_log_query");return}}
  tail,err:=strconv.Atoi(q.Get("tail"))
  if err!=nil||tail<1||tail>MaxLogTail {deny(w,"logs_tail_required_1_1000");return}
  if v:=q.Get("since");v!="" {if _,err:=strconv.ParseInt(v,10,64);err!=nil{deny(w,"invalid_since");return}}
  query:=url.Values{"stdout":{"1"},"stderr":{"1"},"timestamps":{"1"},"tail":{strconv.Itoa(tail)}}
  if v:=q.Get("since");v!=""{query.Set("since",v)}
  b,status,err:=s.fetch(r.Context(),"/containers/"+url.PathEscape(id)+"/logs",query,1<<20)
  if status==404 {jsonOut(w,404,map[string]string{"error":"not_found"});return}
  if err!=nil||status!=200 {jsonOut(w,502,map[string]string{"error":"docker_unavailable"});return}
  lines:=parseDockerLogs(b)
  jsonOut(w,200,map[string]any{"lines":lines,"truncated":false})
 default:
  deny(w,"path_not_allowed")
 }
}

type LogLine struct{Stream string `json:"stream"`; Text string `json:"text"`}
func parseDockerLogs(b []byte)[]LogLine{
 result:=make([]LogLine,0)
 // Docker multiplexed stream: 8-byte header followed by body.
 if len(b)>7&&(b[0]==1||b[0]==2)&&b[1]==0&&b[2]==0&&b[3]==0{
  for len(b)>=8{
   n:=int(binary.BigEndian.Uint32(b[4:8]))
   stream:="stdout";if b[0]==2{stream="stderr"}
   b=b[8:]
   if n<0||n>len(b){break}
   result=append(result,LogLine{Stream:stream,Text:string(b[:n])})
   b=b[n:]
  }
 }else {
  result=append(result,LogLine{Stream:"combined",Text:string(b)})
 }
 return result
}

func (s Service) String()string{return fmt.Sprintf("MackNa Docker gate %p",s.Docker)}
