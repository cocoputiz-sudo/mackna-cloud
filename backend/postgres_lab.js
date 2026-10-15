"use strict";
// All untrusted database values are rendered as textContent, never HTML.
const $ = (id) => document.getElementById(id);
const state = {schema:"public", table:"", view:"rows", page:0, cursors:[null], busy:false, generation:0};
function status(id, message, error=false) {
 const el=$(id); el.textContent=message; el.classList.toggle("error", error);
}
function clear(el) {el.replaceChildren();}
function text(tag, value) {const el=document.createElement(tag);el.textContent=value==null?"—":String(value);return el;}
function empty(parent,message){clear(parent);const el=text("div",message);el.className="empty";parent.append(el);}
function formatBytes(value) {
 const n=Number(value);
 if (!Number.isFinite(n)) return "—";
 if (n<1024) return n+" B";
 const units=["KiB","MiB","GiB"];let v=n;
 for(const unit of units){v/=1024;if(v<1024)return v.toFixed(1)+" "+unit;}
 return (v/1024).toFixed(1)+" TiB";
}
function formatUptime(value) {
 const secs=Number(value);
 if (!Number.isFinite(secs))return "—";
 const d=Math.floor(secs/86400), h=Math.floor(secs%86400/3600),m=Math.floor(secs%3600/60);
 return (d?d+"d ":"")+(h?h+"h ":"")+m+"min";
}
async function api(path) {
 const response=await fetch("/api/v1/postgres/lab/"+path,{cache:"no-store",credentials:"same-origin",signal:AbortSignal.timeout(18000)});
 let data;
 try{data=await response.json();}catch(_){throw new Error("Resposta inválida do servidor (HTTP "+response.status+")");}
 if(!response.ok)throw new Error("HTTP "+response.status+(data?.error?" · "+data.error:""));
 return data;
}
function renderMetrics(info){
 const entries=[
 ["PostgreSQL", info.version],
 ["Em funcionamento",formatUptime(info.uptime_seconds)],
 ["Tamanho do banco",formatBytes(info.database_bytes)],
 ["Conexões",info.connections],
 ["Acertos do cache",Number(info.cache_hit_percent).toFixed(2)+"%"]
 ];
 const parent=$("metrics");clear(parent);
 for(const [label,value] of entries){
  const card=document.createElement("div");card.className="metric";
  card.append(text("span",label),text("strong",value));parent.append(card);
 }
}
async function loadMetrics(){
 status("metrics-status","Consultando...");
 try{renderMetrics(await api("overview"));status("metrics-status","Métricas atualizadas.");}
 catch(e){status("metrics-status","Falha ao carregar métricas: "+e.message,true);}
}
function renderTable(columns,rows){
 const area=$("catalog-results");clear(area);
 if(!rows.length){empty(area,"Nenhum resultado para esta consulta.");return;}
 const wrap=document.createElement("div");wrap.className="table-scroll";
 const table=document.createElement("table"),thead=document.createElement("thead"),tr=document.createElement("tr");
 for(const col of columns)tr.append(text("th",col));
 thead.append(tr);table.append(thead);
 const tbody=document.createElement("tbody");
 for(const row of rows){
  const r=document.createElement("tr");
  for(const value of row){
   let printable=value;
   if(value!==null && typeof value==="object")printable=JSON.stringify(value);
   r.append(text("td",printable));
  }
  tbody.append(r);
 }
 table.append(tbody);wrap.append(table);area.append(wrap);
}
function setOptions(id,items,preferred){
 const select=$(id);clear(select);
 for(const name of items){
  const option=document.createElement("option");
  option.value=name;option.textContent=name;select.append(option);
 }
 if(items.includes(preferred))select.value=preferred;
 return select.value||"";
}
function queryPath(){
 const params=new URLSearchParams({schema:state.schema,table:state.table});
 if(state.view==="rows"){
  params.set("limit","20");
  const cursor=state.cursors[state.page];
  if(cursor!==null&&cursor!==undefined)params.set("after",String(cursor));
 }
 return state.view+"?"+params.toString();
}
function resetPagination(){state.page=0;state.cursors=[null];}
function updateViews(){
 for(const button of document.querySelectorAll("[data-view]")){
  button.setAttribute("aria-pressed",button.dataset.view===state.view?"true":"false");
 }
}
function drawPager(next){
 const isRows=state.view==="rows";
 $("pager").hidden=!isRows;
 $("prev-page").disabled=!isRows||state.page===0;
 $("next-page").disabled=!isRows||next===null||next===undefined;
 $("page-label").textContent="Página "+(state.page+1);
}
async function showView(){
 const generation=++state.generation;
 updateViews();
 if(!state.table){empty($("catalog-results"),"Nenhuma tabela disponível neste schema.");$("pager").hidden=true;status("catalog-status","Sem tabelas.");return;}
 status("catalog-status","Consultando "+state.schema+"."+state.table+"...");
 $("pager").hidden=true;
 try{
  const data=await api(queryPath());
  if(generation!==state.generation)return;
  if(state.view==="rows"){
   renderTable(data.columns||[],data.rows||[]);
   const rows=data.rows||[];
   const next=rows.length===20?data.next_key:null;
   state.nextKey=next;
   drawPager(next);
   status("catalog-status",rows.length+" registro(s) nesta página.");
  }else{
   const names={columns:["Coluna","Tipo","Pode ser nula","Chave primária"],indexes:["Índice","Definição"],"foreign-keys":["Relacionamento","Definição"]};
   const mapped=(Array.isArray(data)?data:[]).map(item=>{
    if(state.view==="columns")return [item.name,item.type,item.nullable?"Sim":"Não",item.primary_key?"Sim":"Não"];
    return [item.name,item.definition];
   });
   renderTable(names[state.view]||[],mapped);
   status("catalog-status",mapped.length+" item(ns) encontrados.");
  }
 }catch(e){
  if(generation!==state.generation)return;
  empty($("catalog-results"),"Não foi possível apresentar a consulta.");
  status("catalog-status","Falha ao consultar: "+e.message,true);
 }
}
async function loadTables(){
 state.generation++;
 state.table="";
 resetPagination();
 try{
  status("catalog-status","Listando tabelas...");
  const data=await api("tables?"+new URLSearchParams({schema:state.schema}));
  const tableNames=(Array.isArray(data)?data:[]).map(row=>row.name);
  state.table=setOptions("table-select",tableNames,tableNames.includes("mackna_fixture")?"mackna_fixture":tableNames[0]);
  await showView();
 }catch(e){
  empty($("catalog-results"),"Não foi possível listar tabelas.");
  status("catalog-status","Falha ao listar tabelas: "+e.message,true);
 }
}
async function loadCatalog(){
 try{
  status("catalog-status","Carregando schemas...");
  const schemas=await api("schemas");
  const names=Array.isArray(schemas)?schemas:[];
  state.schema=setOptions("schema-select",names,state.schema);
  await loadTables();
 }catch(e){
  empty($("catalog-results"),"Não foi possível carregar o catálogo.");
  status("catalog-status","Falha ao carregar catálogo: "+e.message,true);
 }
}
$("refresh-metrics").addEventListener("click",loadMetrics);
$("refresh-catalog").addEventListener("click",loadCatalog);
$("schema-select").addEventListener("change",event=>{state.schema=event.target.value;loadTables();});
$("table-select").addEventListener("change",event=>{state.table=event.target.value;resetPagination();showView();});
document.querySelectorAll("[data-view]").forEach(button=>{
 button.addEventListener("click",()=>{state.view=button.dataset.view;resetPagination();showView();});
});
$("prev-page").addEventListener("click",()=>{if(state.page>0){state.page--;showView();}});
$("next-page").addEventListener("click",()=>{
 if(state.nextKey===null||state.nextKey===undefined)return;
 state.cursors[state.page+1]=state.nextKey;state.page++;showView();
});
loadMetrics();
loadCatalog();
