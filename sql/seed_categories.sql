BEGIN;

INSERT INTO service_categories (name, description)
SELECT name, description
FROM (VALUES
  ('appliance_installation', 'Appliance setup and home device installation.'),
  ('carpenter', 'Furniture assembly, doors and small wood repairs.'),
  ('cleaner', 'Apartment, house or office cleaning.'),
  ('electrician', 'Sockets, lighting, wiring and diagnostics.'),
  ('gardener', 'Garden and plant care.'),
  ('mover', 'Loading, carrying and moving help.'),
  ('plumber', 'Pipes, leaks, mixers and plumbing.'),
  ('renovation', 'Finishing and renovation work.')
) AS defaults(name, description)
WHERE NOT EXISTS (
  SELECT 1 FROM service_categories sc WHERE sc.name = defaults.name
);

COMMIT;
