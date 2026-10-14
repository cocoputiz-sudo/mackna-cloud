package auth

import (
 "os"
 "path/filepath"
 "strings"
 "testing"
)
func TestCreateAuthenticateRevoke(t *testing.T){
 s:=&Store{File:filepath.Join(t.TempDir(),"tokens.json")}
 plain,tk,err:=s.Create("claude",[]string{Read,Logs})
 if err!=nil{t.Fatal(err)}
 b,err:=os.ReadFile(s.File);if err!=nil{t.Fatal(err)}
 if strings.Contains(string(b),plain){t.Fatal("plaintext persisted")}
 st,err:=os.Stat(s.File);if err!=nil||st.Mode().Perm()!=0600{t.Fatal("invalid permission")}
 got,ok:=s.Authenticate(plain)
 if !ok||!got.Has(Read)||!got.Has(Logs)||got.Has(Admin){t.Fatal("scopes invalid")}
 if _,ok:=s.Authenticate(plain+"X");ok{t.Fatal("invalid credential accepted")}
 list,err:=s.List();if err!=nil||len(list)!=1||list[0].Hash!=""{t.Fatal("hash must not be exposed by list")}
 if err:=s.Revoke(tk.ID);err!=nil{t.Fatal(err)}
 if _,ok:=s.Authenticate(plain);ok{t.Fatal("revoked token accepted")}
}
func TestRejectWeakScopeOrPermissions(t *testing.T){
 s:=&Store{File:filepath.Join(t.TempDir(),"tokens.json")}
 if _,_,err:=s.Create("x",[]string{"root"});err==nil{t.Fatal("unsafe scope allowed")}
 plain,_,err:=s.Create("valid",[]string{Admin});if err!=nil{t.Fatal(err)}
 if err:=os.Chmod(s.File,0644);err!=nil{t.Fatal(err)}
 if _,ok:=s.Authenticate(plain);ok{t.Fatal("token store with open permissions accepted")}
}
