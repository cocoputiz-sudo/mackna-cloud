// Package audit provides a small redacted, append-only event trail for
// MackNa control-plane reads. No SQL, query strings or bearer tokens are stored.
package audit

import (
 "bufio"
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "sync"
 "time"
)

type Event struct {
 Time time.Time `json:"time"`
 Action string `json:"action"`
 TokenID string `json:"token_id,omitempty"`
 Path string `json:"path"`
 Status int `json:"status"`
}

type Log struct {
 mu sync.Mutex
 path string
 file *os.File
 size int64
 maxBytes int64
}

func Open(path string,maxBytes int64)(*Log,error){
 if maxBytes<1024{return nil,errors.New("audit maxBytes too small")}
 if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return nil,err}
 f,err:=os.OpenFile(path,os.O_CREATE|os.O_APPEND|os.O_WRONLY,0600)
 if err!=nil{return nil,err}
 if err:=f.Chmod(0600);err!=nil{f.Close();return nil,err}
 st,err:=f.Stat()
 if err!=nil{f.Close();return nil,err}
 return &Log{path:path,file:f,size:st.Size(),maxBytes:maxBytes},nil
}
func (l *Log) rotate()error{
 if err:=l.file.Close();err!=nil{return err}
 if err:=os.Rename(l.path,l.path+".1");err!=nil{return err}
 f,err:=os.OpenFile(l.path,os.O_CREATE|os.O_APPEND|os.O_WRONLY,0600)
 if err!=nil{return err}
 l.file=f;l.size=0
 return nil
}
func (l *Log) Write(ev Event)error{
 if l==nil{return nil}
 if ev.Time.IsZero(){ev.Time=time.Now().UTC()}
 // Reject unusually long metadata; no request parameters are recorded.
 if len(ev.Action)>80||len(ev.Path)>256||len(ev.TokenID)>64{return errors.New("audit metadata too long")}
 body,err:=json.Marshal(ev)
 if err!=nil{return err}
 body=append(body,'\n')
 l.mu.Lock();defer l.mu.Unlock()
 if l.file==nil{return errors.New("audit log closed")}
 if l.size+int64(len(body))>l.maxBytes{
  if err:=l.rotate();err!=nil{return fmt.Errorf("audit rotation: %w",err)}
 }
 n,err:=l.file.Write(body)
 l.size+=int64(n)
 return err
}
func (l *Log) Tail(limit int)([]Event,error){
 if l==nil{return []Event{},nil}
 if limit<1||limit>100{return nil,errors.New("invalid audit limit")}
 l.mu.Lock();defer l.mu.Unlock()
 if l.file==nil{return nil,errors.New("audit log closed")}
 f,err:=os.Open(l.path);if err!=nil{return nil,err}
 defer f.Close()
 sc:=bufio.NewScanner(f)
 sc.Buffer(make([]byte,4096),65536)
 latest:=make([]Event,0,limit)
 for sc.Scan(){
  var ev Event
  if json.Unmarshal(sc.Bytes(),&ev)!=nil{continue}
  if len(latest)==limit{copy(latest,latest[1:]);latest=latest[:limit-1]}
  latest=append(latest,ev)
 }
 if err:=sc.Err();err!=nil{return nil,err}
 for i,j:=0,len(latest)-1;i<j;i,j=i+1,j-1{latest[i],latest[j]=latest[j],latest[i]}
 return latest,nil
}
func (l *Log) Close()error{
 if l==nil{return nil}
 l.mu.Lock();defer l.mu.Unlock()
 if l.file==nil{return nil}
 err:=l.file.Close();l.file=nil;return err
}
