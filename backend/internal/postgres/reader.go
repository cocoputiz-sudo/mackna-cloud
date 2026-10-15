package postgres

import (
 "context"
 "errors"
 "fmt"
 "regexp"
 "time"

 "github.com/jackc/pgx/v5"
 "github.com/jackc/pgx/v5/pgxpool"
)

const MaxRows = 1000
const MaxBytes = 5 << 20

var identifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

type Instance struct {
 ID string
 DSN string
}
type Overview struct {
 Version string `json:"version"`
 UptimeSeconds int64 `json:"uptime_seconds"`
 DatabaseBytes int64 `json:"database_bytes"`
 Connections int64 `json:"connections"`
 CacheHitPercent float64 `json:"cache_hit_percent"`
}
type Table struct {
 Schema string `json:"schema"`
 Name string `json:"name"`
 EstimatedRows int64 `json:"estimated_rows"`
}
type Column struct {
 Name string `json:"name"`
 Type string `json:"type"`
 Nullable bool `json:"nullable"`
 PrimaryKey bool `json:"primary_key"`
}
type Index struct {
 Name string `json:"name"`
 Definition string `json:"definition"`
}
type ForeignKey struct {
 Name string `json:"name"`
 Definition string `json:"definition"`
}
type Page struct {
 Columns []string `json:"columns"`
 Rows [][]any `json:"rows"`
 NextKey any `json:"next_key,omitempty"`
}

type Reader struct { pool *pgxpool.Pool }

func Open(ctx context.Context, dsn string) (*Reader,error) {
 cfg,err:=pgxpool.ParseConfig(dsn)
 if err!=nil{return nil,err}
 cfg.MaxConns=3
 cfg.MinConns=0
 cfg.MaxConnIdleTime=30*time.Second
 cfg.ConnConfig.DefaultQueryExecMode=pgx.QueryExecModeExec
 pool,err:=pgxpool.NewWithConfig(ctx,cfg)
 if err!=nil{return nil,err}
 return &Reader{pool:pool},nil
}
func (r *Reader) Close(){r.pool.Close()}

func (r *Reader) transaction(ctx context.Context, fn func(pgx.Tx)error)error{
 ctx,cancel:=context.WithTimeout(ctx,15*time.Second)
 defer cancel()
 tx,err:=r.pool.BeginTx(ctx,pgx.TxOptions{AccessMode:pgx.ReadOnly})
 if err!=nil{return err}
 defer tx.Rollback(ctx)
 return fn(tx)
}
func (r *Reader) Overview(ctx context.Context)(out Overview,err error){
 err=r.transaction(ctx,func(tx pgx.Tx)error{
  return tx.QueryRow(ctx,`SELECT current_setting('server_version'), extract(epoch from (now()-pg_postmaster_start_time()))::bigint,
  pg_database_size(current_database()),
  (SELECT count(*) FROM pg_stat_activity WHERE datname=current_database()),
  COALESCE((SELECT round(100.0*sum(blks_hit)/nullif(sum(blks_hit)+sum(blks_read),0),2) FROM pg_stat_database WHERE datname=current_database()),0)`).Scan(&out.Version,&out.UptimeSeconds,&out.DatabaseBytes,&out.Connections,&out.CacheHitPercent)
 })
 return
}
func (r *Reader) Schemas(ctx context.Context)(out []string,err error){
 err=r.transaction(ctx,func(tx pgx.Tx)error{
  rows,e:=tx.Query(ctx,`SELECT nspname FROM pg_namespace WHERE nspname NOT LIKE 'pg_%' AND nspname <> 'information_schema' ORDER BY nspname LIMIT 100`)
  if e!=nil{return e};defer rows.Close()
  for rows.Next(){var s string;if e=rows.Scan(&s);e!=nil{return e};out=append(out,s)}
  return rows.Err()
 });return
}
func (r *Reader) Tables(ctx context.Context,schema string)(out []Table,err error){
 if !identifier.MatchString(schema){return nil,errors.New("invalid schema")}
 err=r.transaction(ctx,func(tx pgx.Tx)error{
  rows,e:=tx.Query(ctx,`SELECT n.nspname,c.relname,COALESCE(c.reltuples,0)::bigint FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relkind IN ('r','p') ORDER BY c.relname LIMIT 500`,schema)
  if e!=nil{return e};defer rows.Close()
  for rows.Next(){var t Table;if e=rows.Scan(&t.Schema,&t.Name,&t.EstimatedRows);e!=nil{return e};out=append(out,t)}
  return rows.Err()
 });return
}
func (r *Reader) Columns(ctx context.Context,schema,table string)(out []Column,err error){
 if !identifier.MatchString(schema)||!identifier.MatchString(table){return nil,errors.New("invalid identifier")}
 err=r.transaction(ctx,func(tx pgx.Tx)error{
  rows,e:=tx.Query(ctx,`SELECT a.attname,format_type(a.atttypid,a.atttypmod),NOT a.attnotnull,COALESCE(i.indisprimary AND a.attnum=ANY(i.indkey),false) FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace LEFT JOIN pg_index i ON i.indrelid=c.oid AND i.indisprimary WHERE n.nspname=$1 AND c.relname=$2 AND a.attnum>0 AND NOT a.attisdropped ORDER BY a.attnum LIMIT 500`,schema,table)
  if e!=nil{return e};defer rows.Close()
  for rows.Next(){var col Column;if e=rows.Scan(&col.Name,&col.Type,&col.Nullable,&col.PrimaryKey);e!=nil{return e};out=append(out,col)}
  return rows.Err()
 });return
}
func (r *Reader) Indexes(ctx context.Context,schema,table string)(out []Index,err error){
 if !identifier.MatchString(schema)||!identifier.MatchString(table){return nil,errors.New("invalid identifier")}
 err=r.transaction(ctx,func(tx pgx.Tx)error{
  rows,e:=tx.Query(ctx,`SELECT indexname,indexdef FROM pg_indexes WHERE schemaname=$1 AND tablename=$2 ORDER BY indexname LIMIT 500`,schema,table)
  if e!=nil{return e};defer rows.Close()
  for rows.Next(){var v Index;if e=rows.Scan(&v.Name,&v.Definition);e!=nil{return e};out=append(out,v)}
  return rows.Err()
 });return
}
func (r *Reader) ForeignKeys(ctx context.Context,schema,table string)(out []ForeignKey,err error){
 if !identifier.MatchString(schema)||!identifier.MatchString(table){return nil,errors.New("invalid identifier")}
 err=r.transaction(ctx,func(tx pgx.Tx)error{
  rows,e:=tx.Query(ctx,`SELECT con.conname,pg_get_constraintdef(con.oid) FROM pg_constraint con JOIN pg_class c ON c.oid=con.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2 AND con.contype='f' ORDER BY con.conname LIMIT 500`,schema,table)
  if e!=nil{return e};defer rows.Close()
  for rows.Next(){var v ForeignKey;if e=rows.Scan(&v.Name,&v.Definition);e!=nil{return e};out=append(out,v)}
  return rows.Err()
 });return
}

// Rows uses keyset pagination on a single-column primary key. Tables without
// such a key are deliberately not browsable in Phase A.
func (r *Reader) Rows(ctx context.Context,schema,table string,after any,limit int)(out Page,err error){
 if !identifier.MatchString(schema)||!identifier.MatchString(table){return out,errors.New("invalid identifier")}
 if limit<1||limit>MaxRows{return out,errors.New("invalid limit")}
 err=r.transaction(ctx,func(tx pgx.Tx)error{
  var key string
  e:=tx.QueryRow(ctx,`SELECT a.attname FROM pg_index i JOIN pg_class c ON c.oid=i.indrelid JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum=i.indkey[0] WHERE n.nspname=$1 AND c.relname=$2 AND i.indisprimary AND i.indnkeyatts=1`,schema,table).Scan(&key)
  if e!=nil{return fmt.Errorf("single-column primary key required: %w",e)}
  sql:=`SELECT * FROM `+pgx.Identifier{schema,table}.Sanitize()
  args:=[]any{}
  if after!=nil{sql+=` WHERE `+pgx.Identifier{key}.Sanitize()+` > $1`;args=append(args,after)}
  sql+=` ORDER BY `+pgx.Identifier{key}.Sanitize()+fmt.Sprintf(" LIMIT %d",limit)
  rows,e:=tx.Query(ctx,sql,args...)
  if e!=nil{return e};defer rows.Close()
  fields:=rows.FieldDescriptions()
  keyIndex:=-1
  for i,f:=range fields{out.Columns=append(out.Columns,string(f.Name));if string(f.Name)==key{keyIndex=i}}
  size:=0
  for rows.Next(){
   vals,e:=rows.Values();if e!=nil{return e}
   size+=len(fmt.Sprint(vals))
   if size>MaxBytes{return errors.New("result exceeds 5 MiB")}
   out.Rows=append(out.Rows,vals)
   if keyIndex>=0{out.NextKey=vals[keyIndex]}
  }
  return rows.Err()
 })
 return
}
