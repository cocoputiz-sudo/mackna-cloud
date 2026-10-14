package gate

import (
 "context"
 "encoding/json"
 "io"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
)

type fnTransport func(*http.Request)(*http.Response,error)
func (f fnTransport)RoundTrip(r *http.Request)(*http.Response,error){return f(r)}
func TestGateSanitizesInspect(t *testing.T){
 var upstream []string
 cli:=&http.Client{Transport:fnTransport(func(r *http.Request)(*http.Response,error){
  upstream=append(upstream,r.Method+" "+r.URL.String())
  var body string
  switch r.URL.Path {
  case "/containers/x/json":
   body=`{"Id":"xxxxxxxxxxxxxxxx","Name":"/app","State":{"Status":"running","Running":true},"Config":{"Image":"app:latest","Env":["DISCORD_TOKEN=SECRET"],"Cmd":["--password","SECRET"],"Labels":{"com.docker.compose.project":"test","other.secret":"SECRET"}},"Mounts":[{"Source":"/home/secret","Destination":"/data"}]}`
  case "/containers/json":
   body=`[{"Id":"xxxxxxxxxxxxxxxx","Names":["/app"],"Image":"app","Status":"Up (healthy)","State":"running","Labels":{"com.docker.compose.project":"test","my.secret":"SECRET"}}]`
  case "/_ping":body="OK"
  default:body="{}"
  }
  return &http.Response{StatusCode:200,Body:io.NopCloser(strings.NewReader(body)),Header:http.Header{}},nil
 })}
 gate:=Service{Docker:cli}
 for _,route:=range []string{"/containers/x/json","/containers/json?all=1"}{
  r:=httptest.NewRequest("GET",route,nil);w:=httptest.NewRecorder();gate.ServeHTTP(w,r)
  if w.Code!=200 {t.Fatalf("%s status %d: %s",route,w.Code,w.Body.String())}
  if strings.Contains(w.Body.String(),"SECRET")||strings.Contains(w.Body.String(),"/home/secret")||strings.Contains(w.Body.String(),"Env"){t.Fatalf("secrets leaked on %s: %s",route,w.Body.String())}
  var decoded any
  if err:=json.Unmarshal(w.Body.Bytes(),&decoded);err!=nil{t.Fatal(err)}
 }
 if len(upstream)!=2{t.Fatalf("unexpected upstream calls %v",upstream)}
}
func TestGateAllowlist(t *testing.T){
 upstream:=0
 gate:=Service{Docker:&http.Client{Transport:fnTransport(func(*http.Request)(*http.Response,error){
  upstream++;return &http.Response{StatusCode:200,Body:io.NopCloser(strings.NewReader("{}")),Header:http.Header{}},nil
 })}}
 cases:=[]struct{method,path string}{
  {"POST","/containers/x/restart"},{"DELETE","/containers/x"},
  {"GET","/containers/x/archive?path=/etc"},{"GET","/containers/x/stats?stream=true"},
  {"GET","/containers/x/logs?tail=1001"},{"GET","/containers/x/logs?tail=200&follow=true"},
  {"GET","/containers/..%2Fetc/json"},{"GET","/images/json"},
 }
 for _,tc:=range cases{
  r:=httptest.NewRequest(tc.method,tc.path,nil);w:=httptest.NewRecorder();gate.ServeHTTP(w,r)
  if w.Code!=403{t.Errorf("%s %s status %d",tc.method,tc.path,w.Code)}
 }
 if upstream!=0{t.Fatal("denied requests reached Docker")}
}
func TestGateDockerUnavailable(t *testing.T){
 cli:=&http.Client{Transport:fnTransport(func(*http.Request)(*http.Response,error){return nil,context.DeadlineExceeded})}
 gate:=Service{Docker:cli}
 w:=httptest.NewRecorder();gate.ServeHTTP(w,httptest.NewRequest("GET","/_ping",nil))
 if w.Code!=502{t.Fatalf("status %d",w.Code)}
}
