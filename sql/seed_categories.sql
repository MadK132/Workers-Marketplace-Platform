BEGIN;

INSERT INTO service_categories (name, description)
VALUES
  ('Plumbing', 'Pipes, leaks, bathroom and kitchen fixes.'),
  ('Electrical', 'Wiring, sockets, lights and diagnostics.'),
  ('Cleaning', 'Home and office cleaning services.'),
  ('Carpentry', 'Furniture assembly and wood repairs.'),
  ('Painting', 'Walls, ceilings and decorative painting.')
ON CONFLICT DO NOTHING;

COMMIT;
