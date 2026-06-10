# Service Endpoints and Architecture Explanation

This document explains the main backend services used in the project: User Management, Chat, and Notification. It also explains related technologies such as gRPC, Redis, MinIO/S3, PostgreSQL/PostGIS, JWT, WebSocket, and how they fit into the system.

The project is a geolocation-based workers marketplace. Customers can search for nearby workers, create service requests, chat, pay, report issues, and review completed work. Workers can manage profiles, verify skills, receive bookings, navigate to customers, and communicate in real time.

## Common Backend Pattern

Most services use the same structure:

- `router`: declares HTTP endpoints.
- `handler`: parses HTTP input, validates basic request data, returns JSON responses.
- `service`: contains business logic.
- `repository`: performs PostgreSQL queries.
- `model`: contains Go structs used by the service.
- `middleware`: validates JWT or gateway/internal secret.

The frontend usually calls the API Gateway using `/api/...`. The API Gateway validates JWT for protected routes and forwards the request to the correct microservice.

Public auth endpoints use `/auth/...` directly through the User Management service.

Protected internal service-to-service calls use either:

- gRPC with `x-gateway-secret` metadata.
- HTTP internal endpoints protected by gateway shared secret.

## User Management Service

### Purpose

The User Management service is responsible for:

- registration and login;
- JWT access/refresh tokens;
- email verification;
- password reset;
- customer and worker profiles;
- worker identity documents;
- worker skills and skill upgrades;
- admin/manager review queues;
- reports and penalties;
- saved payment methods;
- staff account creation;
- soft-delete/anonymization of users.

It is one of the central services because many other services need user/profile information.

### Main Technologies

- Go + Gin for HTTP API.
- PostgreSQL for users, profiles, reports, penalties, skills.
- JWT for authentication.
- bcrypt for password hashing.
- MinIO/S3 for uploaded files.
- Resend email API for verification/reset emails.
- gRPC server for internal profile/payment-method lookup.

### Auth Endpoints

`POST /auth/register`

Creates a new customer or worker account.

Flow:

1. Validate full name, email, password, role, and phone.
2. Hash password with bcrypt.
3. Create user with `inactive` status.
4. Generate email verification token.
5. Store token in `email_verifications`.
6. Send verification email.
7. User becomes active only after email verification.

Important tables:

- `users`
- `email_verifications`
- `customer_profiles` or `worker_profiles`

`GET /auth/verify?token=...`

Verifies email.

Flow:

1. Find token in `email_verifications`.
2. Check `expires_at`.
3. Activate user.
4. Delete used token.

`POST /auth/login`

Logs user in.

Flow:

1. Find user by email.
2. Expire outdated penalties if needed.
3. Check active access penalties.
4. Require `status = active`.
5. Compare password using bcrypt.
6. Generate JWT access token and refresh token.

Current JWT TTL is configured in `.env` as `JWT_TTL=15m`.

`POST /auth/refresh`

Creates a new access token using refresh token. It also checks that the user is still active and not blocked.

`POST /auth/resend-verification`

Deletes old verification tokens for the user and creates a new verification token.

`POST /auth/forgot-password`

Creates a password reset token and sends reset email. It does not reveal whether the email exists, which is good security behavior.

`GET /auth/reset`

Serves/reset page for password reset token.

`POST /auth/reset-password`

Changes password using reset token.

Flow:

1. Find reset token in `password_resets`.
2. Check `expires_at`.
3. Hash new password.
4. Update user password.
5. Delete used reset token.

### Customer Profile Endpoints

`POST /api/customer/profile`

Creates or updates customer profile.

Stores:

- address;
- latitude/longitude;
- geography point;
- bio/about notes;
- profile photo URL.

`POST /api/customer/profile/photo`

Uploads profile photo independently from saving profile data. File is stored in MinIO/S3. Database stores only file path/URL.

`GET /api/customer/profile`

Returns current customer profile.

### Worker Profile Endpoints

`GET /api/worker/profile`

Returns current worker profile, verified skills, identity document status, rating, photo, and verification status.

`POST /api/worker/profile`

Creates or updates worker bio and current location.

`POST /api/worker/profile/photo`

Uploads worker profile photo to MinIO/S3.

`PATCH /api/worker/availability`

Turns worker online/offline.

Important logic:

- Worker can go online only when identity document and at least one skill are verified.
- Active penalties can block going online.
- Worker profile has `is_available` and `verification_status`.

### Worker Verification Endpoints

`POST /api/worker/identity-document`

Uploads ID card/passport evidence.

Flow:

1. File is validated by extension/size.
2. File is stored in MinIO/S3.
3. `worker_identity_documents` row is created with `pending` status.
4. Admin/manager can review it.

`POST /api/worker/skills`

Worker submits a skill/service with evidence.

Stores:

- category;
- experience level;
- base price;
- evidence note/files;
- `is_verified = false` until staff approves.

`POST /api/worker/skill-upgrades`

Worker requests upgrade from junior to middle/senior etc. Current service remains active while upgrade is pending.

### Admin/Manager Verification Endpoints

`GET /api/admin/overview`

Returns admin dashboard statistics and review queues:

- pending identities;
- pending skills;
- pending upgrades;
- users count;
- bookings count;
- in-progress bookings.

`POST /api/admin/assign-identity-document`

Manager can take an identity document case.

`POST /api/admin/verify-identity-document`

Approves identity document. Worker may become verified if skills are also verified.

`POST /api/admin/reject-identity-document`

Rejects identity document with reason.

`POST /api/admin/verify-skill`

Approves worker skill.

`POST /api/admin/reject-skill`

Rejects submitted skill.

`POST /api/admin/verify-skill-upgrade`

Approves upgrade request and updates worker skill level.

`POST /api/admin/reject-skill-upgrade`

Rejects upgrade request.

### Reports and Penalties

Reports are handled in User Management service.

`POST /api/reports`

Creates a support report.

Flow:

1. Customer/worker selects booking or reported user.
2. Report reason and description are stored.
3. Optional proof files are uploaded to MinIO/S3.
4. Notifications are created for staff and reported user.

`GET /api/reports`

Lists visible reports.

Visibility:

- customer/worker sees own reports;
- manager sees unassigned or assigned-to-me reports;
- admin sees all reports.

`GET /api/reports/{report_id}/messages`

Lists support conversation messages for one report.

There are two conversation sides:

- reporter side;
- reported side.

This allows support staff to speak separately with reporter and reported user.

`POST /api/reports/{report_id}/messages`

Adds message or file to report chat.

`PATCH /api/admin/reports/{report_id}/assign`

Assigns report to current manager/admin.

`POST /api/admin/reports/{report_id}/penalty`

Applies penalty.

Supported penalties:

- `warning`;
- `temporary_suspend`;
- `block_user`;
- `unverify_skills`.

Important algorithm:

- For `unverify_skills`, only the skill related to the reported booking category is unverified.
- It does not randomly unverify all worker skills.

`PATCH /api/admin/penalties/{penalty_id}/cancel`

Cancels active penalty. For block/suspend, service checks whether another active blocking penalty still exists.

`PATCH /api/admin/reports/{report_id}/close`

Closes or rejects report without applying penalty.

### Admin Users and Soft Delete

`GET /api/admin/users`

Lists non-deleted users.

`GET /api/admin/users/{id}/profile`

Returns customer/worker profile details for staff.

`POST /api/admin/admins`

Creates admin account.

`POST /api/admin/managers`

Creates manager account.

`PATCH /api/admin/users/{id}/activate`

Activates user manually.

`DELETE /api/admin/users/{id}`

Soft-deletes/anonymizes user.

Current behavior:

- user row is kept for historical foreign keys;
- `deleted_at` is set;
- `full_name` becomes `Deleted user`;
- email becomes `deleted-user-{id}@deleted.local`;
- phone/password are cleared;
- status becomes `inactive`;
- chats, notifications, reset/verification tokens, saved cards are deleted;
- worker identity files, skills, upgrades are deleted;
- worker availability is turned off;
- bookings, reports, reviews, payments, penalties and service requests remain as history.

Reason:

This preserves audit/history while removing the user from active product workflows.

### User Management gRPC

Defined in:

`api/usermanagement-service-proto/usermanagement.proto`

RPC methods:

- `GetCustomerProfile(user_id)` -> returns `customer_profile_id`.
- `GetWorkerProfile(user_id)` -> returns `worker_profile_id`.
- `HasPaymentMethod(user_id)` -> returns true/false.

Used by other services, especially Booking service, to check profile/payment prerequisites without exposing internal database logic over public HTTP.

gRPC calls are protected by shared secret metadata, using an interceptor.

## Chat Service

### Purpose

The Chat service provides booking-based communication between customer and worker.

It supports:

- creating/opening a chat for a booking;
- listing chats;
- sending text messages;
- sending file attachments;
- marking messages as read;
- real-time WebSocket updates;
- Redis Pub/Sub for multi-instance real-time delivery;
- notifications for new chat messages.

### Main Technologies

- Go + Gin.
- PostgreSQL for chats/messages.
- WebSocket using Gorilla WebSocket.
- Redis Pub/Sub for broadcasting messages between app instances.
- MinIO/S3 for attachments.
- HTTP client to Notification service.

### Chat Endpoints

`POST /api/chats`

Creates or returns chat for a booking.

Input:

- booking id;
- customer user id;
- worker user id.

Algorithm:

1. Validate current user access.
2. Check booking participants.
3. Insert chat.
4. If chat already exists for booking, return existing chat.

Database rule:

- `booking_id` is unique in `chats`, so one booking has one chat.

`GET /api/chats`

Lists chats where current user is customer or worker.

Also returns unread count:

```sql
COUNT(messages where sender_user_id != current_user and read_at is null)
```

`GET /api/chats/{chat_id}/messages`

Lists messages for a chat.

Supports:

- `limit`, default 50;
- `before_id` for pagination.

Algorithm:

1. Check current user belongs to chat.
2. Query messages ordered by latest id descending.
3. Reverse result in memory before returning, so frontend receives chronological order.

`POST /api/chats/{chat_id}/messages`

Sends message.

Supports:

- JSON text message;
- multipart message with file attachment.

Attachment validation:

- allowed: jpg, jpeg, png, webp, mp4, mov, webm, pdf, doc, docx, txt;
- max size: 12 MB.

Flow:

1. Validate chat access.
2. Check chat is active.
3. Check booking is not completed/cancelled.
4. Store file in MinIO/S3 if provided.
5. Insert message into PostgreSQL.
6. Update chat `updated_at`.
7. Broadcast event to local WebSocket clients.
8. Publish event to Redis for other app instances.
9. Create notification for recipient.

`PATCH /api/chats/{chat_id}/read`

Marks all messages from the other user as read.

`GET /api/chats/{chat_id}/ws`

Opens WebSocket connection.

WebSocket client can send:

```json
{
  "type": "message",
  "content": "Hello"
}
```

Server broadcasts:

```json
{
  "type": "message.created",
  "chat_id": 1,
  "message": { "...": "..." }
}
```

### Chat Closing Algorithm

The repository has `syncClosedChats`.

Before listing or using chats, it updates:

- chat status -> `closed`
- when booking status is `completed` or `cancelled`

This means after a booking is finished/cancelled, chat becomes read-only/history.

### Redis in Chat

Redis is used as Pub/Sub bus.

Why:

If the application runs multiple chat-service instances, WebSocket clients may be connected to different instances. Without Redis, a message sent on instance A would only be broadcast to clients connected to instance A.

How it works:

1. Instance A saves message to PostgreSQL.
2. Instance A broadcasts to its local WebSocket clients.
3. Instance A publishes event to Redis channel.
4. Other instances subscribe to Redis channel.
5. Instance B receives event from Redis and broadcasts to its local WebSocket clients.

The event includes `node_id`. If an instance receives its own event from Redis, it ignores it to prevent duplicate messages.

If Redis is disabled/unavailable, the service can still work locally with `NoopPublisher`, but multi-instance real-time delivery is lost.

## Notification Service

### Purpose

Notification service stores user notifications.

It supports:

- creating notification from internal services;
- listing user notifications;
- marking one as read;
- marking all as read.

Notifications can also include navigation action metadata, for example:

- action type: `chat`, `report`, `booking`;
- action ref: id of target entity;
- action label: text shown in UI.

### Technologies

- Go + Gin.
- PostgreSQL.
- Gateway shared secret middleware.

### Endpoints

`POST /internal/notifications`

Internal endpoint for services to create notification.

Input:

- `user_id`
- `type`
- `title`
- `message`
- `action_type`
- `action_ref`
- `action_label`

Used by:

- Chat service after new message;
- report logic in User Management service;
- other flows that need persisted notifications.

`GET /api/notifications`

Lists notifications for current user.

Query params:

- `limit`, default 50;
- `unread=true` to return only unread notifications.

Cleanup algorithm:

- when listing notifications, old read notifications are deleted if `read_at < now - 7 days`.

`PATCH /api/notifications/{notification_id}/read`

Marks one notification as read.

`PATCH /api/notifications/read-all`

Marks all unread user notifications as read.

### Notification Table

Fields:

- `notification_id`
- `user_id`
- `type`
- `title`
- `message`
- `action_type`
- `action_ref`
- `action_label`
- `is_read`
- `read_at`
- `created_at`

## gRPC Overview

The project uses gRPC for internal service-to-service communication.

Why gRPC:

- strong contracts through `.proto` files;
- faster and more structured than ad-hoc JSON for internal calls;
- generated Go clients/servers;
- better separation between public API and internal API.

Existing gRPC services:

### UserManagementService

File:

`api/usermanagement-service-proto/usermanagement.proto`

Methods:

- `GetCustomerProfile`
- `GetWorkerProfile`
- `HasPaymentMethod`

Used by Booking service to check whether a user has required profile/payment data.

### GeolocationService

File:

`api/geolocation-service-proto/geolocation.proto`

Method:

- `FindNearbyWorkers`

Input:

- category id;
- latitude;
- longitude;
- radius in meters.

Output:

- list of nearby workers with coordinates and distance.

### BookingService

File:

`api/booking-service-proto/booking.proto`

Methods include:

- create request;
- list customer requests;
- create booking;
- list customer bookings;
- list worker bookings;
- start booking;
- complete booking.

### PaymentService

File:

`api/payment-service-proto/payment.proto`

Methods include:

- create payment;
- get payment;
- mark payment completed;
- mark payment failed.

### gRPC Security

Internal gRPC servers use a shared-secret interceptor. The caller sends a secret in metadata. The server rejects calls without the correct secret.

This is not user authentication. It is service-to-service authentication.

## MinIO / S3

MinIO is an S3-compatible object storage.

Used for:

- profile photos;
- worker identity documents;
- worker skill evidence;
- chat attachments;
- report files;
- review photos.

Why not store files directly in PostgreSQL:

- DB stays smaller and faster;
- files can be served separately;
- object storage is better for large binary files;
- it follows common cloud architecture patterns.

Typical flow:

1. Frontend sends multipart upload.
2. Backend validates extension and size.
3. Backend uploads file to MinIO/S3 bucket.
4. Backend stores returned path/URL in PostgreSQL.
5. Frontend later opens file using that path/URL.

Main env variables:

- `S3_ENDPOINT`
- `S3_ACCESS_KEY`
- `S3_SECRET_KEY`
- `S3_BUCKET`
- `S3_PUBLIC_URL`
- `S3_USE_SSL`

## PostgreSQL and PostGIS

PostgreSQL is the main relational database.

PostGIS extension is used for geolocation.

Why PostGIS:

- supports geographic coordinates;
- supports distance calculations;
- supports indexes for nearby search;
- avoids manual inaccurate distance filtering in application code.

Important PostGIS functions used:

`ST_SetSRID(ST_MakePoint(longitude, latitude), 4326)::geography`

Creates a geography point from latitude/longitude.

Important: point order is longitude, latitude.

`ST_DWithin(worker_location, origin_point, radius_meters)`

Checks if worker is inside radius.

`ST_Distance(worker_location, origin_point)`

Computes distance in meters.

### Worker Search Algorithm

Used by Geolocation service.

Input:

- category id;
- customer latitude;
- customer longitude;
- radius meters.

Algorithm:

1. Build origin geography point from customer location.
2. Join `worker_profiles`, `users`, `worker_skills`, `service_categories`.
3. Filter:
   - selected category;
   - skill is verified;
   - user is active;
   - worker profile is verified;
   - worker is available;
   - worker has current location;
   - worker is inside search radius using `ST_DWithin`.
4. Compute distance using `ST_Distance`.
5. Sort by distance, then rating.

This is why nearby search is fast and accurate.

## WebSocket

WebSocket is used in Chat service for real-time messages.

Why WebSocket:

- HTTP request/response is not enough for instant incoming messages.
- WebSocket keeps a persistent connection open.
- Server can push new messages to frontend immediately.

Flow:

1. Frontend opens `/api/chats/{chat_id}/ws`.
2. Server validates user access to chat.
3. Server registers client in in-memory Hub.
4. When a new message is saved, Hub broadcasts to all clients connected to that chat.
5. Redis forwards event to other service instances.

The frontend also has REST polling fallback, so chat remains usable even if WebSocket fails.

## JWT Authentication

JWT is used for user authentication.

Flow:

1. User logs in.
2. User Management service creates JWT access token.
3. Frontend sends token in `Authorization: Bearer <token>`.
4. API Gateway and services parse token.
5. Claims provide:
   - user id;
   - role;
   - expiration time.

JWT is stateless. That means a token remains valid until expiry unless services check database state on every request.

Current access token TTL:

- `JWT_TTL=15m`

## API Gateway

API Gateway exposes `/api/*`.

Responsibilities:

- validate JWT for protected API routes;
- forward requests to correct microservice;
- pass user id/role through headers;
- centralize frontend entry point.

Why gateway is useful:

- frontend does not need to know every microservice URL;
- easier auth enforcement;
- cleaner routing;
- microservices stay internal.

## Report/Notification/Chat Interaction Example

Example: customer sends a chat message.

1. Frontend sends `POST /api/chats/{chat_id}/messages`.
2. Chat service checks access.
3. Message is saved in `chat_messages`.
4. Chat `updated_at` is updated.
5. Message is broadcast over local WebSocket hub.
6. Message event is published to Redis.
7. Other chat-service instances receive Redis event and broadcast to their clients.
8. Chat service creates notification for recipient through Notification service.
9. Recipient sees both real-time message and persisted notification.

## Worker Verification Example

Example: worker adds new skill.

1. Worker sends `POST /api/worker/skills`.
2. Files are uploaded to MinIO/S3.
3. Skill is inserted as `is_verified = false`.
4. Evidence rows are created.
5. Admin/manager sees it in review queue.
6. Staff approves with `POST /api/admin/verify-skill`.
7. Skill becomes visible to customers.
8. If identity document is also verified, worker can go online.

## User Delete Example

Current delete is not hard delete.

Admin presses Delete:

1. System snapshots report participant name/email.
2. Deletes active chat/notification/reset/payment-method data.
3. Clears private profile data.
4. Deletes worker verification files/skills/upgrades.
5. Turns worker unavailable.
6. Anonymizes user row and sets `deleted_at`.
7. Keeps bookings, reviews, reports, payments, penalties, service requests as history.

This keeps audit/history for admin and payment/report integrity.

## What To Say In Defense

Short explanation:

The system uses a microservice architecture. User Management owns identity, roles, profiles, verification, reports, penalties, and staff workflows. Chat owns booking-based messaging and real-time delivery. Notification owns persistent user notifications. PostgreSQL stores structured business data. PostGIS provides spatial search. MinIO stores uploaded files. Redis is used as a Pub/Sub layer for real-time chat events across multiple chat-service instances. gRPC is used for internal service-to-service calls with typed contracts and shared-secret protection. JWT is used for user authentication through the API Gateway.

One good sentence:

"The architecture separates user identity, real-time communication, notifications, booking, geolocation and payment responsibilities into independent services, while PostgreSQL/PostGIS, Redis and MinIO provide specialized storage and infrastructure for relational data, spatial search, real-time event fan-out and file storage."

