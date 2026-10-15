package main
import (
 "embed"
 "net/http"
)
//go:embed audit.html audit.js
var auditAssets embed.FS
func (c *controller) serveAuditAsset(w http.ResponseWriter,r *http.Request){
 name:="audit.html";mime:="text/html; charset=utf-8"
 if r.URL.Path=="/audit.js"{name="audit.js";mime="text/javascript; charset=utf-8"}
 bytes,err:=auditAssets.ReadFile(name)
 if err!=nil{http.Error(w,"asset_unavailable",http.StatusServiceUnavailable);return}
 w.Header().Set("Content-Type",mime)
 _,_=w.Write(bytes)
}
