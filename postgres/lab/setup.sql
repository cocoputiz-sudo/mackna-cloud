-- Only for disposable CI database mackna_lab.
CREATE ROLE mackna_reader LOGIN PASSWORD 'ci-lab-reader';
GRANT pg_read_all_data TO mackna_reader;
ALTER ROLE mackna_reader SET default_transaction_read_only = on;
ALTER ROLE mackna_reader SET statement_timeout = '15s';
ALTER ROLE mackna_reader SET work_mem = '4MB';
ALTER ROLE mackna_reader SET idle_in_transaction_session_timeout = '30s';
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
CREATE TABLE public.mackna_fixture(id bigint PRIMARY KEY, label text NOT NULL);
INSERT INTO public.mackna_fixture VALUES (1,'one'),(2,'two');
CREATE TABLE public.mackna_side_effect(id integer PRIMARY KEY);
CREATE FUNCTION public.mackna_write_attempt() RETURNS integer LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO public.mackna_side_effect VALUES (1);
 RETURN 1;
END $$;
