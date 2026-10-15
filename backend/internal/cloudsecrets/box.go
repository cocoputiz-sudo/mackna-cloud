// Package cloudsecrets provides a fail-closed encryption primitive for the
// future MackNa v2 deployer. It is not enabled in the existing controller.
package cloudsecrets

import (
 "crypto/aes"
 "crypto/cipher"
 "crypto/rand"
 "encoding/base64"
 "errors"
 "fmt"
 "io"
 "os"
 "path/filepath"
)

const prefix="mkenc:v1:"
type Box struct{ aead cipher.AEAD }

func NewBox(key []byte)(*Box,error){
 if len(key)!=32{return nil,errors.New("AES-256 requires 32-byte key")}
 block,err:=aes.NewCipher(key);if err!=nil{return nil,err}
 aead,err:=cipher.NewGCM(block);if err!=nil{return nil,err}
 return &Box{aead:aead},nil
}

func (b *Box) Seal(plain string)(string,error){
 if b==nil{return "",errors.New("encryption key unavailable")}
 nonce:=make([]byte,b.aead.NonceSize())
 if _,err:=io.ReadFull(rand.Reader,nonce);err!=nil{return "",err}
 sealed:=b.aead.Seal(nil,nonce,[]byte(plain),[]byte(prefix))
 raw:=append(nonce,sealed...)
 return prefix+base64.RawStdEncoding.EncodeToString(raw),nil
}

func (b *Box) Open(value string)(string,error){
 if b==nil{return "",errors.New("encryption key unavailable")}
 if len(value)<=len(prefix)||value[:len(prefix)]!=prefix{
  return "",errors.New("encrypted value required")
 }
 raw,err:=base64.RawStdEncoding.DecodeString(value[len(prefix):])
 if err!=nil{return "",errors.New("invalid ciphertext")}
 size:=b.aead.NonceSize()
 if len(raw)<size+b.aead.Overhead(){return "",errors.New("truncated ciphertext")}
 plain,err:=b.aead.Open(nil,raw[:size],raw[size:],[]byte(prefix))
 if err!=nil{return "",errors.New("authentication failed")}
 return string(plain),nil
}

func readExisting(path string)(*Box,error){
 info,err:=os.Lstat(path);if err!=nil{return nil,err}
 if !info.Mode().IsRegular() || info.Mode().Perm()!=0600 {
  return nil,errors.New("master key must be a regular 0600 file")
 }
 // Opening an already-present file is only for a private trusted directory.
 key,err:=os.ReadFile(path);if err!=nil{return nil,err}
 return NewBox(key)
}

// LoadOrCreateKey uses an exclusively created temporary file + hard link:
// concurrent initializers cannot overwrite each other's master key.
// The key must be backed up separately before any data encrypted by it.
func LoadOrCreateKey(path string)(*Box,error){
 if b,err:=readExisting(path);err==nil{return b,nil}else if !errors.Is(err,os.ErrNotExist){return nil,err}
 dir:=filepath.Dir(path)
 if err:=os.MkdirAll(dir,0700);err!=nil{return nil,err}
 key:=make([]byte,32)
 if _,err:=io.ReadFull(rand.Reader,key);err!=nil{return nil,err}
 temp,err:=os.CreateTemp(dir,".mk-key-*")
 if err!=nil{return nil,err}
 tmpName:=temp.Name()
 defer os.Remove(tmpName)
 if err:=temp.Chmod(0600);err!=nil{temp.Close();return nil,err}
 if _,err:=temp.Write(key);err!=nil{temp.Close();return nil,err}
 if err:=temp.Sync();err!=nil{temp.Close();return nil,err}
 if err:=temp.Close();err!=nil{return nil,err}
 if err:=os.Link(tmpName,path);err!=nil{
  if errors.Is(err,os.ErrExist){return readExisting(path)}
  return nil,fmt.Errorf("create master key: %w",err)
 }
 return NewBox(key)
}
