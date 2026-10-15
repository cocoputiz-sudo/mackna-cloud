package postgres

import (
 "context"
 "errors"
 "fmt"

 "github.com/jackc/pgx/v5"
)

// FixedReport is deliberately restricted to catalog/statistics reads. It never
// executes SQL received from a user and never returns another session's query text.
type FixedReport struct {
 Columns []string `json:"columns"`
 Rows [][]any `json:"rows"`
}

// Only trusted built-in SQL strings are executable; the HTTP caller supplies
// the report key, never SQL or SQL parameters.
var safeReportSQL = map[string]string{
 "connections": `SELECT COALESCE(state,'unknown') AS state, COALESCE(wait_event_type,'none') AS wait_type, count(*)::bigint AS sessions
   FROM pg_stat_activity WHERE datname=current_database()
   GROUP BY state,wait_event_type ORDER BY sessions DESC LIMIT 30`,
 "table-stats": `SELECT schemaname,relname,n_live_tup,n_dead_tup,
   COALESCE(seq_scan,0),COALESCE(idx_scan,0),last_autovacuum,last_autoanalyze
   FROM pg_stat_user_tables ORDER BY n_live_tup DESC,relname LIMIT 100`,
 "index-stats": `SELECT schemaname,relname,indexrelname,COALESCE(idx_scan,0),
   pg_relation_size(indexrelid)::bigint AS index_bytes
   FROM pg_stat_user_indexes ORDER BY idx_scan DESC,indexrelname LIMIT 100`,
 "locks": `SELECT locktype,mode,granted,count(*)::bigint AS locks
   FROM pg_locks WHERE pid IN (SELECT pid FROM pg_stat_activity WHERE datname=current_database())
   GROUP BY locktype,mode,granted ORDER BY locks DESC LIMIT 40`,
 "database-stats": `SELECT numbackends,xact_commit,xact_rollback,blks_read,blks_hit,
   tup_returned,tup_fetched,tup_inserted,tup_updated,tup_deleted,deadlocks,temp_bytes
   FROM pg_stat_database WHERE datname=current_database()`,
 "settings": `SELECT name,setting,COALESCE(unit,'') AS unit
   FROM pg_settings WHERE name IN ('max_connections','shared_buffers',
   'work_mem','maintenance_work_mem','effective_cache_size','max_wal_size',
   'statement_timeout','idle_in_transaction_session_timeout','lock_timeout',
   'autovacuum','timezone','server_version','ssl')
   ORDER BY name LIMIT 50`,
 "extensions": `SELECT extname,extversion,n.nspname AS schema
   FROM pg_extension e JOIN pg_namespace n ON n.oid=e.extnamespace
   ORDER BY extname LIMIT 100`,
}

// ReportNames returns a fixed, predictable list for the browser.
func ReportNames() []string {
 return []string{"connections","table-stats","index-stats","locks","database-stats","settings","extensions"}
}

var ErrUnknownReport = errors.New("unknown_report")

func (r *Reader) NamedReport(ctx context.Context, name string) (out FixedReport, err error) {
 query, ok := safeReportSQL[name]
 if !ok {return out,ErrUnknownReport}
 out.Rows = make([][]any,0)
 err = r.transaction(ctx,func(tx pgx.Tx) error {
  rows,e:=tx.Query(ctx,query)
  if e!=nil{return e}
  defer rows.Close()
  for _,fd:=range rows.FieldDescriptions(){out.Columns=append(out.Columns,string(fd.Name))}
  bytesSeen:=0
  for rows.Next(){
   if len(out.Rows)>=100 {return errors.New("report_row_limit_exceeded")}
   vals,e:=rows.Values()
   if e!=nil{return e}
   bytesSeen+=len(fmt.Sprint(vals))
   if bytesSeen>512<<10{return errors.New("report_size_limit_exceeded")}
   out.Rows=append(out.Rows,vals)
  }
  return rows.Err()
 })
 return out,err
}
