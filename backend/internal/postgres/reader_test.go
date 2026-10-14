package postgres

import (
 "context"
 "os"
 "testing"
 "time"

 "github.com/jackc/pgx/v5"
)

func TestLabReadOnly(t *testing.T){
 dsn:=os.Getenv("MACKNA_LAB_DSN")
 if dsn==""{t.Skip("lab not configured")}
 ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second);defer cancel()
 reader,err:=Open(ctx,dsn);if err!=nil{t.Fatal(err)}
 defer reader.Close()
 overview,err:=reader.Overview(ctx);if err!=nil||overview.Version==""{t.Fatalf("overview: %+v %v",overview,err)}
 schemas,err:=reader.Schemas(ctx);if err!=nil||len(schemas)==0{t.Fatalf("schemas: %v %v",schemas,err)}
 tables,err:=reader.Tables(ctx,"public");if err!=nil||len(tables)==0{t.Fatalf("tables: %v %v",tables,err)}
 cols,err:=reader.Columns(ctx,"public","mackna_fixture");if err!=nil||len(cols)!=2{t.Fatalf("columns: %v %v",cols,err)}
 idx,err:=reader.Indexes(ctx,"public","mackna_fixture");if err!=nil||len(idx)==0{t.Fatalf("indexes: %v %v",idx,err)}
 _,err=reader.ForeignKeys(ctx,"public","mackna_fixture");if err!=nil{t.Fatal(err)}
 page,err:=reader.Rows(ctx,"public","mackna_fixture",nil,1);if err!=nil||len(page.Rows)!=1||page.NextKey==nil{t.Fatalf("first page: %+v %v",page,err)}
 page2,err:=reader.Rows(ctx,"public","mackna_fixture",page.NextKey,1);if err!=nil||len(page2.Rows)!=1{t.Fatalf("second page: %+v %v",page2,err)}
 if _,err=reader.Rows(ctx,"public","mackna_fixture",nil,1001);err==nil{t.Fatal("limit must be rejected")}
 conn,err:=pgx.Connect(ctx,dsn);if err!=nil{t.Fatal(err)}
 defer conn.Close(ctx)
 for name,query:=range map[string]string{
  "insert":"INSERT INTO public.mackna_fixture VALUES (3,'denied')",
  "update":"UPDATE public.mackna_fixture SET label='denied'",
  "delete":"DELETE FROM public.mackna_fixture",
  "create":"CREATE TABLE public.mackna_denied(id int)",
  "function":"SELECT public.mackna_write_attempt()",
 }{
  t.Run(name,func(t *testing.T){if _,err:=conn.Exec(ctx,query);err==nil{t.Fatalf("%s unexpectedly succeeded",name)}})
 }
}
