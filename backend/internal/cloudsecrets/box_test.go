package cloudsecrets

import (
 "os"
 "path/filepath"
 "strings"
 "sync"
 "testing"
)

func TestSealOpenAuthenticatedAndRandomized(t *testing.T){
 key:=[]byte(strings.Repeat("k",32))
 box,err:=NewBox(key);if err!=nil{t.Fatal(err)}
 one,err:=box.Seal("DATABASE_URL=postgres://test@local")
 if err!=nil{t.Fatal(err)}
 two,err:=box.Seal("DATABASE_URL=postgres://test@local")
 if err!=nil{t.Fatal(err)}
 if one==two{t.Fatal("nonce reuse: identical ciphertext")}
 if strings.Contains(one,"postgres") {t.Fatal("plaintext visible")}
 for _,sealed:=range []string{one,two}{
  plain,err:=box.Open(sealed)
  if err!=nil||plain!="DATABASE_URL=postgres://test@local"{t.Fatalf("round trip failed: %v",err)}
 }
 raw:=[]byte(one)
 raw[len(raw)-3]^=1
 if _,err:=box.Open(string(raw));err==nil{t.Fatal("modified ciphertext accepted")}
 for _,bad:=range []string{"","plaintext","enc:v1:abc","mkenc:v1:","mkenc:v1:a"}{
  if _,err:=box.Open(bad);err==nil{t.Fatalf("accepted %q",bad)}
 }
 second,_:=NewBox([]byte(strings.Repeat("x",32)))
 if _,err:=second.Open(one);err==nil{t.Fatal("wrong key accepted")}
 if _,err:=NewBox([]byte("short"));err==nil{t.Fatal("short key accepted")}
}

func TestKeyAtomicConcurrentAndPermissions(t *testing.T){
 path:=filepath.Join(t.TempDir(),"secret","master.key")
 const n=16
 boxes:=make([]*Box,n)
 errs:=make([]error,n)
 var wg sync.WaitGroup
 for i:=0;i<n;i++{wg.Add(1);go func(i int){defer wg.Done();boxes[i],errs[i]=LoadOrCreateKey(path)}(i)}
 wg.Wait()
 for _,err:=range errs{if err!=nil{t.Fatal(err)}}
 token,err:=boxes[0].Seal("same-key");if err!=nil{t.Fatal(err)}
 for i:=1;i<n;i++{plain,err:=boxes[i].Open(token);if err!=nil||plain!="same-key"{t.Fatalf("different master key from concurrent loader %d: %v",i,err)}}
 st,err:=os.Lstat(path);if err!=nil{t.Fatal(err)}
 if st.Mode().Perm()!=0600{t.Fatalf("master key permissions: %v",st.Mode())}
 files,err:=os.ReadDir(filepath.Dir(path));if err!=nil{t.Fatal(err)}
 if len(files)!=1{t.Fatalf("temporary keys left behind: %d",len(files))}
}

func TestKeyRejectsInsecureFilesAndSymlinks(t *testing.T){
 for _,mode:=range []os.FileMode{0644,0660,0777}{
  p:=filepath.Join(t.TempDir(),"key")
  if err:=os.WriteFile(p,[]byte(strings.Repeat("q",32)),mode);err!=nil{t.Fatal(err)}
  if err:=os.Chmod(p,mode);err!=nil{t.Fatal(err)}
  if _,err:=LoadOrCreateKey(p);err==nil{t.Fatalf("accepted insecure key mode %o",mode)}
 }
 dir:=t.TempDir()
 actual:=filepath.Join(dir,"actual")
 link:=filepath.Join(dir,"linked")
 if err:=os.WriteFile(actual,[]byte(strings.Repeat("x",32)),0600);err!=nil{t.Fatal(err)}
 if err:=os.Symlink(actual,link);err!=nil{t.Fatal(err)}
 if _,err:=LoadOrCreateKey(link);err==nil{t.Fatal("symlink key accepted")}
 if err:=os.WriteFile(filepath.Join(dir,"corrupt"),[]byte("short"),0600);err!=nil{t.Fatal(err)}
 if _,err:=LoadOrCreateKey(filepath.Join(dir,"corrupt"));err==nil{t.Fatal("corrupt existing key was silently replaced")}
}
