CREATE EXTENSION IF NOT EXISTS postgis;

ALTER TABLE customer_profiles
  ADD COLUMN IF NOT EXISTS location geography(Point, 4326);

ALTER TABLE worker_profiles
  ADD COLUMN IF NOT EXISTS current_location geography(Point, 4326);

ALTER TABLE service_requests
  ADD COLUMN IF NOT EXISTS location geography(Point, 4326);

UPDATE customer_profiles
SET location = ST_SetSRID(ST_MakePoint(longitude::float8, latitude::float8), 4326)::geography
WHERE latitude IS NOT NULL
  AND longitude IS NOT NULL
  AND location IS NULL;

UPDATE worker_profiles
SET current_location = ST_SetSRID(ST_MakePoint(current_longitude::float8, current_latitude::float8), 4326)::geography
WHERE current_latitude IS NOT NULL
  AND current_longitude IS NOT NULL
  AND current_location IS NULL;

UPDATE service_requests
SET location = ST_SetSRID(ST_MakePoint(longitude::float8, latitude::float8), 4326)::geography
WHERE latitude IS NOT NULL
  AND longitude IS NOT NULL
  AND location IS NULL;

CREATE INDEX IF NOT EXISTS idx_customer_profiles_location
  ON customer_profiles USING GIST (location);

CREATE INDEX IF NOT EXISTS idx_worker_profiles_current_location
  ON worker_profiles USING GIST (current_location);

CREATE INDEX IF NOT EXISTS idx_service_requests_location
  ON service_requests USING GIST (location);
