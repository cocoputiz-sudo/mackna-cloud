"use strict";
const $=id=>document.getElementById(id);
function cell(v){const td=document.createElement("td");td.textContent=v==null?"—":String(v);return td;}
async function loadAudit(){
 $("audit-status").textContent="Consultando...";
 try{
  const response=await fetch("/api/v1/audit?limit=50",{cache:"no-store",credentials:"same-origin"});
  if(!response.ok)throw new Error("HTTP "+response.status);
  const data=await response.json();
  $("audit-events").replaceChildren();
  for(const item of (data.entries||[])){
   const row=document.createElement("tr");
   row.append(cell(item.time),cell(item.action),cell(item.token_id),cell(item.path),cell(item.status));
   $("audit-events").append(row);
  }
  $("audit-status").textContent=(data.entries||[]).length+" evento(s) exibidos.";
 }catch(err){$("audit-status").textContent="Falha: "+err.message;$("audit-status").classList.add("error");}
}
$("refresh-audit").addEventListener("click",loadAudit);
loadAudit();
