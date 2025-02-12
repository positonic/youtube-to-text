CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS "Video" (
    id TEXT PRIMARY KEY,
    "videoUrl" TEXT NOT NULL,
    transcription TEXT,
    status TEXT NOT NULL,
    "isSearchable" BOOLEAN DEFAULT false,
    "createdAt" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    "userId" INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS video_chunks (
    id SERIAL PRIMARY KEY,
    video_id TEXT REFERENCES "Video"(id),
    chunk_text TEXT NOT NULL,
    chunk_embedding vector(1536),
    chunk_start INTEGER,
    chunk_end INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX ON video_chunks 
USING ivfflat (chunk_embedding vector_cosine_ops)
WITH (lists = 100);

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW."updatedAt" = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_video_updated_at
    BEFORE UPDATE ON "Video"
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Add notification trigger for new videos
CREATE OR REPLACE FUNCTION notify_new_video()
  RETURNS trigger AS $$
BEGIN
  PERFORM pg_notify('new_video', row_to_json(NEW)::text);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER video_inserted_trigger
  AFTER INSERT ON "Video"
  FOR EACH ROW
  EXECUTE FUNCTION notify_new_video();

\dt
\dx
