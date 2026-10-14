// Package auth implements scoped high-entropy bearer tokens.
// Only SHA-256 of a 256-bit random secret is retained on disk.
package auth

import (
 "crypto/rand"
 "crypto/sha256"
 "crypto/subtle"
 "encoding/base64"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "regexp"
 "strings"
 "sync"
 "time"
)
const (Read="read"; Logs="logs"; Admin="admin")
type Token struct {
 ID string `json:"id"`
 Name string `json:"name"`
 Scopes []string `json:"scopes"`
 Hash string `json:"hash"`
 Created time.Time `json:"created"`
 Revoked bool `json:"revoked"`
}
func (t Token) Has(scope string)bool{
 for _,v:=range t.Scopes {if v==scope||v==Admin {return true}}
 return false
}
type storeFile struct{Version int `json:"version"`; Tokens []Token `json:"tokens"`}
type Store struct{File string; Mu sync.Mutex}
var pattern=regexp.MustCompile(`^mkn_([0-9a-f]{12})_([a-zA-Z0-9_-]{43})$`)

func (s *Store) read() (storeFile,error){
 f:=storeFile{Version:1,Tokens:[]Token{}}
 st,err:=os.Stat(s.File)
 if errors.Is(err,os.ErrNotExist){return f,nil}
 if err!=nil{return f,err}
 if st.Mode().Perm()&0077!=0{return f,errors.New("token file permissions must be 0600")}
 b,err:=os.ReadFile(s.File);if err!=nil{return f,err}
 if err=json.Unmarshal(b,&f);err!=nil{return f,err}
 if f.Version!=1{return f,errors.New("unsupported token file version")}
 return f,nil
}
func (s *Store) write(f storeFile)error{
 if err:=os.MkdirAll(filepath.Dir(s.File),0700);err!=nil{return err}
 tmp,err:=os.CreateTemp(filepath.Dir(s.File),".mackna-token-*")
 if err!=nil{return err}
 defer os.Remove(tmp.Name())
 if err=tmp.Chmod(0600);err!=nil{tmp.Close();return err}
 if err=json.NewEncoder(tmp).Encode(f);err!=nil{tmp.Close();return err}
 if err=tmp.Sync();err!=nil{tmp.Close();return err}
 if err=tmp.Close();err!=nil{return err}
 return os.Rename(tmp.Name(),s.File)
}
func validateScopes(v []string)error{
 if len(v)==0{return errors.New("scopes required")}
 for _,s:=range v{if s!=Read&&s!=Logs&&s!=Admin{return fmt.Errorf("unknown scope %q",s)}}
 return nil
}
func (s *Store) Create(name string,scopes []string)(string,Token,error){
 var zero Token
 if len(strings.TrimSpace(name))<1||len(name)>64{return "",zero,errors.New("name must have 1-64 characters")}
 if err:=validateScopes(scopes);err!=nil{return "",zero,err}
 id:=make([]byte,6);secret:=make([]byte,32)
 if _,err:=rand.Read(id);err!=nil{return "",zero,err}
 if _,err:=rand.Read(secret);err!=nil{return "",zero,err}
 encoded:=base64.RawURLEncoding.EncodeToString(secret)
 h:=sha256.Sum256([]byte(encoded))
 token:=Token{ID:hex.EncodeToString(id),Name:name,Scopes:scopes,Hash:hex.EncodeToString(h[:]),Created:time.Now().UTC()}
 s.Mu.Lock();defer s.Mu.Unlock()
 f,err:=s.read();if err!=nil{return "",zero,err}
 for _,v:=range f.Tokens{if v.ID==token.ID{return "",zero,errors.New("token ID collision")}}
 f.Tokens=append(f.Tokens,token)
 if err=s.write(f);err!=nil{return "",zero,err}
 return "mkn_"+token.ID+"_"+encoded,token,nil
}
func (s *Store) Authenticate(value string)(Token,bool){
 s.Mu.Lock();defer s.Mu.Unlock()
 p:=pattern.FindStringSubmatch(value);if p==nil{return Token{},false}
 f,err:=s.read();if err!=nil{return Token{},false}
 h:=sha256.Sum256([]byte(p[2]))
 for _,tok:=range f.Tokens{
  if tok.ID!=p[1] || tok.Revoked{continue}
  b,err:=hex.DecodeString(tok.Hash)
  if err!=nil||len(b)!=32{continue}
  if subtle.ConstantTimeCompare(h[:],b)==1{return tok,true}
 }
 return Token{},false
}
func (s *Store) Revoke(id string)error{
 s.Mu.Lock();defer s.Mu.Unlock()
 f,err:=s.read();if err!=nil{return err}
 for i:=range f.Tokens{
  if f.Tokens[i].ID==id{f.Tokens[i].Revoked=true;return s.write(f)}
 }
 return errors.New("token not found")
}
func (s *Store) List() ([]Token,error){
 s.Mu.Lock();defer s.Mu.Unlock()
 f,err:=s.read()
 for i:=range f.Tokens{f.Tokens[i].Hash=""}
 return f.Tokens,err
}
