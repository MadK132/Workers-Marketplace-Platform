Workers-Marketplace-Platform

## Architecture

```mermaid
flowchart LR
  %% Layers
  subgraph Client["Client Layer"]
    FE["Frontend PWA / Client App"]
  end

  subgraph Gateway["Gateway Layer"]
    GW["API Gateway"]
  end

  subgraph Services["Service Layer"]
    USER["User Management Service\nAuth, profiles, staff, verification, reports"]
    BOOK["Booking Service\nRequests, bookings, reviews, completion"]
    CHAT["Chat Service\nBooking chat, support chat, WebSocket"]
    GEO["Geolocation Service\nNearby workers, worker/customer location"]
    NOTIF["Notification Service\nUser notifications"]
    PAY["Payment Service\nStripe payments / webhooks"]
  end

  subgraph Data["Data Layer"]
    PG[("PostgreSQL + PostGIS")]
    REDIS[("Redis\nchat pub/sub")]
    S3[("MinIO / S3\nprofile photos, evidence, chat files")]
  end

  subgraph External["External Systems"]
    STRIPE["Stripe"]
    GIS["2GIS MapGL / Routing API"]
    RESEND["Resend Email API"]
  end

  FE -->|"HTTP / REST"| GW
  FE -->|"Map tiles, geocoding, routes"| GIS

  GW -->|"HTTP proxy: /auth, default /api"| USER
  GW -->|"HTTP proxy: /api/requests, /api/bookings, /api/reviews"| BOOK
  GW -->|"HTTP proxy: /api/chats, WebSocket"| CHAT
  GW -->|"HTTP proxy: /api/geo"| GEO
  GW -->|"HTTP proxy: /api/notifications"| NOTIF

  BOOK -->|"gRPC: profile/payment checks"| USER
  BOOK -->|"gRPC: create payment"| PAY
  BOOK -->|"HTTP internal notifications"| NOTIF
  CHAT -->|"HTTP internal notifications"| NOTIF
  USER -->|"internal notifications for reports / staff cases"| NOTIF

  PAY -->|"Checkout / payment API"| STRIPE
  STRIPE -->|"webhook events"| PAY
  USER -->|"email verification / reset"| RESEND

  USER --> PG
  BOOK --> PG
  CHAT --> PG
  GEO --> PG
  NOTIF --> PG
  PAY --> PG

  CHAT --> REDIS

  USER --> S3
  BOOK --> S3
  CHAT --> S3
```

### Storage Usage

- `usermanagement-service` stores profile photos, worker skill evidence, identity documents, and report attachments in MinIO/S3.
- `booking-service` stores completion evidence and review photos in MinIO/S3.
- `chat-service` stores chat attachments in MinIO/S3.
- `chat-service` uses Redis for WebSocket pub/sub between service instances.
- `PostgreSQL + PostGIS` is the shared relational and geospatial database.

### Main Communication Types

- Frontend talks to backend through `api-gateway` over HTTP.
- `api-gateway` routes public API calls to the correct service.
- `booking-service` uses gRPC for required internal calls to `usermanagement-service` and `payment-service`.
- `notification-service` is called through internal HTTP endpoints by services that need to create notifications.
- 2GIS is used from the frontend for maps, routing, geocoding, and map rendering.
- Stripe is used by `payment-service` for payments and webhooks.
