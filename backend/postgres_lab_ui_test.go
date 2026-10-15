package main

import (
 "strings"
 "testing"
)

// Regression: the lab console must work under the agent's restrictive CSP.
// Previous builds shipped inline JS and onclick handlers, leaving the UI frozen.
func TestPostgresLabCSPAndAssets(t *testing.T) {
 t.Setenv("MACKNA_PG_LAB_ENABLED", "1")
 c,read,_:=setup(t)
 for _,path:=range []string{"/postgres/lab","/postgres/lab.js"} {
  if got:=req(c,"GET",path,"",false);got.Code!=401{
   t.Fatalf("%s without auth: %d",path,got.Code)
  }
 }
 html:=req(c,"GET","/postgres/lab",read,true)
 if html.Code!=200 {t.Fatalf("HTML status=%d: %s",html.Code,html.Body.String())}
 page:=html.Body.String()
 if !strings.Contains(page,`<script src="/postgres/lab.js" defer></script>`) {
  t.Fatal("lab page must load external JavaScript")
 }
 for _,forbidden:=range []string{"<script>","onclick=", "onload="} {
  if strings.Contains(page,forbidden){t.Fatalf("CSP-incompatible inline code: %q",forbidden)}
 }
 if !strings.Contains(page,`id="refresh-metrics"`) ||
  !strings.Contains(page,`data-view="rows"`) ||
  !strings.Contains(page,`id="schema-select"`) {
  t.Fatal("buttons lack external JS event hooks")
 }
 csp:=html.Header().Get("Content-Security-Policy")
 for _,needed:=range []string{`default-src 'none'`,`script-src 'self'`,`connect-src 'self'`} {
  if !strings.Contains(csp,needed){t.Fatalf("missing CSP permission %q: %s",needed,csp)}
 }
 if strings.Contains(csp,"script-src 'unsafe-inline'"){t.Fatal("inline scripts must remain forbidden")}
 js:=req(c,"GET","/postgres/lab.js",read,true)
 if js.Code!=200 {t.Fatalf("JS status=%d: %s",js.Code,js.Body.String())}
 if !strings.Contains(js.Header().Get("Content-Type"),"javascript"){t.Fatal("incorrect JavaScript MIME")}
 source:=js.Body.String()
 for _,required:=range []string{`addEventListener("click"`,`loadMetrics()`,`loadCatalog()`,`textContent`,`fetch("/api/v1/postgres/lab/"`} {
  if !strings.Contains(source,required){t.Fatalf("missing JS behavior %q",required)}
 }
}

func TestPostgresLabScriptDisabledByDefault(t *testing.T){
 t.Setenv("MACKNA_PG_LAB_ENABLED","0")
 c,read,_:=setup(t)
 for _,path:=range []string{"/postgres/lab","/postgres/lab.js"}{
  r:=req(c,"GET",path,read,true)
  if r.Code!=404{t.Fatalf("%s must stay disabled: %d",path,r.Code)}
 }
}
