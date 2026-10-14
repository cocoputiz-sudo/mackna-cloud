package main

import (
 "context"
 "errors"
 "log"
 "net"
 "net/http"
 "os"
 "os/signal"
 "os/user"
 "path/filepath"
 "strconv"
 "syscall"
 "time"

 "github.com/cocoputiz-sudo/mackna-cloud/backend/internal/gate"
)

func env(key,def string)string{if value:=os.Getenv(key);value!=""{return value};return def}

func main(){
 sock:=env("MACKNA_GATE_LISTEN","/run/mackna-gate/docker-ro.sock")
 target:=env("MACKNA_DOCKER_SOCKET","/var/run/docker.sock")
 group:=env("MACKNA_GATE_CLIENT_GROUP","mackna")
 if err:=os.MkdirAll(filepath.Dir(sock),0755);err!=nil{log.Fatal(err)}
 if st,err:=os.Lstat(sock);err==nil {
  if st.Mode()&os.ModeSocket==0{log.Fatal("refusing to replace non-socket file")}
  if err:=os.Remove(sock);err!=nil{log.Fatal(err)}
 }else if !errors.Is(err,os.ErrNotExist){log.Fatal(err)}
 listener,err:=net.Listen("unix",sock);if err!=nil{log.Fatal(err)}
 defer listener.Close()
 g,err:=user.LookupGroup(group);if err!=nil{log.Fatal(err)}
 gid,err:=strconv.Atoi(g.Gid);if err!=nil{log.Fatal(err)}
 if err:=os.Chown(sock,-1,gid);err!=nil{log.Fatal(err)}
 if err:=os.Chmod(sock,0660);err!=nil{log.Fatal(err)}
 server:=&http.Server{Handler:gate.Service{Docker:gate.DockerClient(target)},ReadHeaderTimeout:5*time.Second,WriteTimeout:22*time.Second,IdleTimeout:30*time.Second}
 done:=make(chan error,1)
 go func(){done<-server.Serve(listener)}()
 log.Printf("MackNa Gate ready (unix socket %s)",sock)
 sig:=make(chan os.Signal,1);signal.Notify(sig,syscall.SIGTERM,syscall.SIGINT)
 select{case err:=<-done:if err!=nil&&!errors.Is(err,http.ErrServerClosed){log.Fatal(err)}
 case <-sig:
  ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel()
  if err:=server.Shutdown(ctx);err!=nil{log.Printf("shutdown: %v",err)}
 }
}
