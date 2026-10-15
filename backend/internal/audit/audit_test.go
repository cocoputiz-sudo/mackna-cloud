package audit

import (
 "os"
 "path/filepath"
 "strings"
 "testing"
)
func TestTrailRedactedRotatedAndBounded(t *testing.T){
 path:=filepath.Join(t.TempDir(),"trail.jsonl")
 l,err:=Open(path,1024)
 if err!=nil{t.Fatal(err)}
 for i:=0;i<35;i++{
  if err:=l.Write(Event{Action:"pg_report",Path:"/api/v1/postgres/lab/report",TokenID:"redacted-id",Status:200});err!=nil{t.Fatal(err)}
 }
 out,err:=l.Tail(5)
 if err!=nil||len(out)!=5{t.Fatalf("tail: %d %v",len(out),err)}
 if _,err:=os.Stat(path+".1");err!=nil{t.Fatal("rotation missing",err)}
 if st,err:=os.Stat(path);err!=nil||st.Mode().Perm()!=0600{t.Fatalf("insecure mode: %v %v",st,err)}
 raw,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)}
 if strings.Contains(string(raw),"Bearer")||strings.Contains(string(raw),"SELECT"){t.Fatal("audit leaked query or secret")}
 if err:=l.Close();err!=nil{t.Fatal(err)}
 if err:=l.Write(Event{Action:"after_close"});err==nil{t.Fatal("write after close accepted")}
}
func TestAuditRejectsOversizedData(t *testing.T){
 l,err:=Open(filepath.Join(t.TempDir(),"a"),1024);if err!=nil{t.Fatal(err)};defer l.Close()
 if err:=l.Write(Event{Action:strings.Repeat("x",81)});err==nil{t.Fatal("oversized event accepted")}
 if _,err:=l.Tail(101);err==nil{t.Fatal("unbounded audit tail accepted")}
}
