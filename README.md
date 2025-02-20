# youtube-to-text

A service that automatically transcribes YouTube videos to text using LemonFox API. It comes with a one-off script to test the process, and a production setup for running the service.

The production setup uses a PostgreSQL database and a trigger to listen for new videos inserted into the database. When a new video is inserted, the service will download the audio, send it to the LemonFox API, and store the transcription in the database.


## Prerequisites

- PostgreSQL database
- LemonFox API key (for transcription service)
- yt-dlp command-line tool (for downloading YouTube audio)
- Go 1.22+

## Setup

### 1. Environment Variables

Create a `.env` file based on `.env.example`.
The application requires the following environment variables to be set in a `.env` file:

If you create a new database, you can set the `DATABASE_URL_<identifier>` environment variable to the database URL.

Then you should run the script as follows:

```bash
go run cmd/transcription/main.go <identifier>
```

for example:

```bash
go run cmd/transcription/main.go DEFAULT
```

will look for an variable in your .env called `DATABASE_URL_DEFAULT` and use that to connect to the database.

### 2. Try a one-off run to see how it works

This uses a hardcoded video URL in the script and log the transcription to the console.

```bash
go run one-off.go
```

## Production Setup

### 3. Database Setup
The code below assumes you have a table named `Video`.

```sql
CREATE TABLE "Video" (
id SERIAL PRIMARY KEY,
url TEXT NOT NULL,
status TEXT DEFAULT 'pending',
transcription TEXT,
created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

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

### 3. Running the Service

1. Start the transcription service:
```bash
go run service.go transcribe.go
```

## Rag search functionality

Make sure you have created a vector database. I used pgvector on railway.app [![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/new/template/3jJFCA)

Run the following set up your database:

```bash
psql "postgres://postgres:xxxxx@xxxxx:37549/railway" -f setup.sql
```

Add a random SERVICE_API_KEY to the .env file.
You can generate a random key with `openssl rand -hex 32`.
