# youtube-to-text

### Create a table in your database

The code below assumes you have a table named `Video`.


### Create the trigger in your database:

```bash
psql "postgresql://username:password@hostname:port/database" -c "
CREATE OR REPLACE FUNCTION notify_new_video()
  RETURNS trigger AS \$\$
BEGIN
  PERFORM pg_notify('new_video', row_to_json(NEW)::text);
  RETURN NEW;
END;
\$\$ LANGUAGE plpgsql;

CREATE TRIGGER video_inserted_trigger
  AFTER INSERT ON \"Video\"
  FOR EACH ROW
  EXECUTE FUNCTION notify_new_video();
"
```

### Insert a record and confirm trigger works

```bash
psql "connection_string" -c "\df notify_new_video"
```

