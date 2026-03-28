--
-- PostgreSQL database dump
--

\restrict WdvlGkvHEpfPVDufYm5aBDf2S14C9N3OB3izhNS5cgHDQaM519IWEdVoKO4zd8f

-- Dumped from database version 18.1
-- Dumped by pg_dump version 18.1

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: booking_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.booking_status AS ENUM (
    'scheduled',
    'in_progress',
    'completed',
    'cancelled'
);


--
-- Name: experience_level; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.experience_level AS ENUM (
    'junior',
    'middle',
    'senior',
    'expert'
);


--
-- Name: payment_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.payment_status AS ENUM (
    'pending',
    'completed',
    'failed',
    'refunded'
);


--
-- Name: request_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.request_status AS ENUM (
    'pending',
    'accepted',
    'in_progress',
    'completed',
    'cancelled'
);


--
-- Name: user_role; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.user_role AS ENUM (
    'customer',
    'worker',
    'admin'
);


--
-- Name: user_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.user_status AS ENUM (
    'active',
    'inactive',
    'banned'
);


--
-- Name: verification_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.verification_status AS ENUM (
    'unverified',
    'pending',
    'verified',
    'rejected'
);


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: bookings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.bookings (
    booking_id integer NOT NULL,
    request_id integer,
    worker_profile_id integer,
    scheduled_time timestamp without time zone,
    start_time timestamp without time zone,
    end_time timestamp without time zone,
    status public.booking_status DEFAULT 'scheduled'::public.booking_status,
    final_price numeric(10,2)
);


--
-- Name: bookings_booking_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.bookings_booking_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: bookings_booking_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.bookings_booking_id_seq OWNED BY public.bookings.booking_id;


--
-- Name: chat_messages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_messages (
    message_id integer NOT NULL,
    chat_id integer,
    sender_user_id integer,
    message_text text,
    sent_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    read_status boolean DEFAULT false
);


--
-- Name: chat_messages_message_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_messages_message_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_messages_message_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_messages_message_id_seq OWNED BY public.chat_messages.message_id;


--
-- Name: chats; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chats (
    chat_id integer NOT NULL,
    booking_id integer
);


--
-- Name: chats_chat_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chats_chat_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chats_chat_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chats_chat_id_seq OWNED BY public.chats.chat_id;


--
-- Name: customer_profiles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.customer_profiles (
    customer_profile_id integer NOT NULL,
    user_id integer,
    address text,
    latitude numeric(9,6),
    longitude numeric(9,6)
);


--
-- Name: customer_profiles_customer_profile_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.customer_profiles_customer_profile_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: customer_profiles_customer_profile_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.customer_profiles_customer_profile_id_seq OWNED BY public.customer_profiles.customer_profile_id;


--
-- Name: notifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notifications (
    notification_id integer NOT NULL,
    user_id integer,
    type character varying(50),
    title character varying(255),
    message text,
    is_read boolean DEFAULT false,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: notifications_notification_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.notifications_notification_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: notifications_notification_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.notifications_notification_id_seq OWNED BY public.notifications.notification_id;


--
-- Name: payments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payments (
    payment_id integer NOT NULL,
    booking_id integer,
    amount numeric(10,2),
    currency character(3) DEFAULT 'USD'::bpchar,
    payment_status public.payment_status DEFAULT 'pending'::public.payment_status,
    payment_method character varying(50),
    transaction_reference character varying(255),
    paid_at timestamp without time zone
);


--
-- Name: payments_payment_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.payments_payment_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: payments_payment_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.payments_payment_id_seq OWNED BY public.payments.payment_id;


--
-- Name: reviews; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reviews (
    review_id integer NOT NULL,
    booking_id integer,
    customer_profile_id integer,
    worker_profile_id integer,
    rating integer,
    comment text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT reviews_rating_check CHECK (((rating >= 1) AND (rating <= 5)))
);


--
-- Name: reviews_review_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.reviews_review_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: reviews_review_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.reviews_review_id_seq OWNED BY public.reviews.review_id;


--
-- Name: service_categories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.service_categories (
    category_id integer NOT NULL,
    name character varying(100) NOT NULL,
    description text
);


--
-- Name: service_categories_category_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.service_categories_category_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: service_categories_category_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.service_categories_category_id_seq OWNED BY public.service_categories.category_id;


--
-- Name: service_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.service_requests (
    request_id integer NOT NULL,
    customer_profile_id integer,
    category_id integer,
    description text,
    address text,
    latitude numeric(9,6),
    longitude numeric(9,6),
    status public.request_status DEFAULT 'pending'::public.request_status,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: service_requests_request_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.service_requests_request_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: service_requests_request_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.service_requests_request_id_seq OWNED BY public.service_requests.request_id;


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    user_id integer NOT NULL,
    full_name character varying(255) NOT NULL,
    email character varying(255) NOT NULL,
    phone character varying(50),
    password_hash character varying(255) NOT NULL,
    role public.user_role NOT NULL,
    status public.user_status DEFAULT 'active'::public.user_status,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: users_user_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.users_user_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: users_user_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.users_user_id_seq OWNED BY public.users.user_id;


--
-- Name: worker_profiles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.worker_profiles (
    worker_profile_id integer NOT NULL,
    user_id integer,
    bio text,
    rating numeric(3,2),
    verification_status public.verification_status DEFAULT 'unverified'::public.verification_status,
    current_latitude numeric(9,6),
    current_longitude numeric(9,6),
    is_available boolean DEFAULT true,
    CONSTRAINT worker_profiles_rating_check CHECK (((rating >= (0)::numeric) AND (rating <= (5)::numeric)))
);


--
-- Name: worker_profiles_worker_profile_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.worker_profiles_worker_profile_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: worker_profiles_worker_profile_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.worker_profiles_worker_profile_id_seq OWNED BY public.worker_profiles.worker_profile_id;


--
-- Name: worker_skills; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.worker_skills (
    worker_skill_id integer NOT NULL,
    worker_profile_id integer,
    category_id integer,
    experience_level public.experience_level NOT NULL,
    price_base numeric(10,2)
);


--
-- Name: worker_skills_worker_skill_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.worker_skills_worker_skill_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: worker_skills_worker_skill_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.worker_skills_worker_skill_id_seq OWNED BY public.worker_skills.worker_skill_id;


--
-- Name: bookings booking_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bookings ALTER COLUMN booking_id SET DEFAULT nextval('public.bookings_booking_id_seq'::regclass);


--
-- Name: chat_messages message_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_messages ALTER COLUMN message_id SET DEFAULT nextval('public.chat_messages_message_id_seq'::regclass);


--
-- Name: chats chat_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chats ALTER COLUMN chat_id SET DEFAULT nextval('public.chats_chat_id_seq'::regclass);


--
-- Name: customer_profiles customer_profile_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_profiles ALTER COLUMN customer_profile_id SET DEFAULT nextval('public.customer_profiles_customer_profile_id_seq'::regclass);


--
-- Name: notifications notification_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications ALTER COLUMN notification_id SET DEFAULT nextval('public.notifications_notification_id_seq'::regclass);


--
-- Name: payments payment_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payments ALTER COLUMN payment_id SET DEFAULT nextval('public.payments_payment_id_seq'::regclass);


--
-- Name: reviews review_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews ALTER COLUMN review_id SET DEFAULT nextval('public.reviews_review_id_seq'::regclass);


--
-- Name: service_categories category_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.service_categories ALTER COLUMN category_id SET DEFAULT nextval('public.service_categories_category_id_seq'::regclass);


--
-- Name: service_requests request_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.service_requests ALTER COLUMN request_id SET DEFAULT nextval('public.service_requests_request_id_seq'::regclass);


--
-- Name: users user_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users ALTER COLUMN user_id SET DEFAULT nextval('public.users_user_id_seq'::regclass);


--
-- Name: worker_profiles worker_profile_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.worker_profiles ALTER COLUMN worker_profile_id SET DEFAULT nextval('public.worker_profiles_worker_profile_id_seq'::regclass);


--
-- Name: worker_skills worker_skill_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.worker_skills ALTER COLUMN worker_skill_id SET DEFAULT nextval('public.worker_skills_worker_skill_id_seq'::regclass);


--
-- Data for Name: bookings; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.bookings (booking_id, request_id, worker_profile_id, scheduled_time, start_time, end_time, status, final_price) FROM stdin;
\.


--
-- Data for Name: chat_messages; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.chat_messages (message_id, chat_id, sender_user_id, message_text, sent_at, read_status) FROM stdin;
\.


--
-- Data for Name: chats; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.chats (chat_id, booking_id) FROM stdin;
\.


--
-- Data for Name: customer_profiles; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.customer_profiles (customer_profile_id, user_id, address, latitude, longitude) FROM stdin;
\.


--
-- Data for Name: notifications; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.notifications (notification_id, user_id, type, title, message, is_read, created_at) FROM stdin;
\.


--
-- Data for Name: payments; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.payments (payment_id, booking_id, amount, currency, payment_status, payment_method, transaction_reference, paid_at) FROM stdin;
\.


--
-- Data for Name: reviews; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.reviews (review_id, booking_id, customer_profile_id, worker_profile_id, rating, comment, created_at) FROM stdin;
\.


--
-- Data for Name: service_categories; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.service_categories (category_id, name, description) FROM stdin;
\.


--
-- Data for Name: service_requests; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.service_requests (request_id, customer_profile_id, category_id, description, address, latitude, longitude, status, created_at) FROM stdin;
\.


--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.users (user_id, full_name, email, phone, password_hash, role, status, created_at) FROM stdin;
\.


--
-- Data for Name: worker_profiles; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.worker_profiles (worker_profile_id, user_id, bio, rating, verification_status, current_latitude, current_longitude, is_available) FROM stdin;
\.


--
-- Data for Name: worker_skills; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.worker_skills (worker_skill_id, worker_profile_id, category_id, experience_level, price_base) FROM stdin;
\.


--
-- Name: bookings_booking_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.bookings_booking_id_seq', 1, false);


--
-- Name: chat_messages_message_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.chat_messages_message_id_seq', 1, false);


--
-- Name: chats_chat_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.chats_chat_id_seq', 1, false);


--
-- Name: customer_profiles_customer_profile_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.customer_profiles_customer_profile_id_seq', 1, false);


--
-- Name: notifications_notification_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.notifications_notification_id_seq', 1, false);


--
-- Name: payments_payment_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.payments_payment_id_seq', 1, false);


--
-- Name: reviews_review_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.reviews_review_id_seq', 1, false);


--
-- Name: service_categories_category_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.service_categories_category_id_seq', 1, false);


--
-- Name: service_requests_request_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.service_requests_request_id_seq', 1, false);


--
-- Name: users_user_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.users_user_id_seq', 1, false);


--
-- Name: worker_profiles_worker_profile_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.worker_profiles_worker_profile_id_seq', 1, false);


--
-- Name: worker_skills_worker_skill_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.worker_skills_worker_skill_id_seq', 1, false);


--
-- Name: bookings bookings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bookings
    ADD CONSTRAINT bookings_pkey PRIMARY KEY (booking_id);


--
-- Name: chat_messages chat_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_messages
    ADD CONSTRAINT chat_messages_pkey PRIMARY KEY (message_id);


--
-- Name: chats chats_booking_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chats
    ADD CONSTRAINT chats_booking_id_key UNIQUE (booking_id);


--
-- Name: chats chats_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chats
    ADD CONSTRAINT chats_pkey PRIMARY KEY (chat_id);


--
-- Name: customer_profiles customer_profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_profiles
    ADD CONSTRAINT customer_profiles_pkey PRIMARY KEY (customer_profile_id);


--
-- Name: customer_profiles customer_profiles_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_profiles
    ADD CONSTRAINT customer_profiles_user_id_key UNIQUE (user_id);


--
-- Name: notifications notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_pkey PRIMARY KEY (notification_id);


--
-- Name: payments payments_booking_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payments
    ADD CONSTRAINT payments_booking_id_key UNIQUE (booking_id);


--
-- Name: payments payments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payments
    ADD CONSTRAINT payments_pkey PRIMARY KEY (payment_id);


--
-- Name: reviews reviews_booking_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT reviews_booking_id_key UNIQUE (booking_id);


--
-- Name: reviews reviews_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT reviews_pkey PRIMARY KEY (review_id);


--
-- Name: service_categories service_categories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.service_categories
    ADD CONSTRAINT service_categories_pkey PRIMARY KEY (category_id);


--
-- Name: service_requests service_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.service_requests
    ADD CONSTRAINT service_requests_pkey PRIMARY KEY (request_id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_phone_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_phone_key UNIQUE (phone);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (user_id);


--
-- Name: worker_profiles worker_profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.worker_profiles
    ADD CONSTRAINT worker_profiles_pkey PRIMARY KEY (worker_profile_id);


--
-- Name: worker_profiles worker_profiles_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.worker_profiles
    ADD CONSTRAINT worker_profiles_user_id_key UNIQUE (user_id);


--
-- Name: worker_skills worker_skills_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.worker_skills
    ADD CONSTRAINT worker_skills_pkey PRIMARY KEY (worker_skill_id);


--
-- Name: idx_bookings_worker; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_bookings_worker ON public.bookings USING btree (worker_profile_id);


--
-- Name: idx_customer_location; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_customer_location ON public.customer_profiles USING btree (latitude, longitude);


--
-- Name: idx_messages_chat_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_messages_chat_id ON public.chat_messages USING btree (chat_id);


--
-- Name: idx_messages_sender; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_messages_sender ON public.chat_messages USING btree (sender_user_id);


--
-- Name: idx_requests_category; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_requests_category ON public.service_requests USING btree (category_id);


--
-- Name: idx_worker_location; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_worker_location ON public.worker_profiles USING btree (current_latitude, current_longitude);


--
-- Name: bookings bookings_request_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bookings
    ADD CONSTRAINT bookings_request_id_fkey FOREIGN KEY (request_id) REFERENCES public.service_requests(request_id) ON DELETE RESTRICT;


--
-- Name: bookings bookings_worker_profile_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bookings
    ADD CONSTRAINT bookings_worker_profile_id_fkey FOREIGN KEY (worker_profile_id) REFERENCES public.worker_profiles(worker_profile_id) ON DELETE RESTRICT;


--
-- Name: chat_messages chat_messages_chat_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_messages
    ADD CONSTRAINT chat_messages_chat_id_fkey FOREIGN KEY (chat_id) REFERENCES public.chats(chat_id) ON DELETE CASCADE;


--
-- Name: chat_messages chat_messages_sender_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_messages
    ADD CONSTRAINT chat_messages_sender_user_id_fkey FOREIGN KEY (sender_user_id) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: chats chats_booking_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chats
    ADD CONSTRAINT chats_booking_id_fkey FOREIGN KEY (booking_id) REFERENCES public.bookings(booking_id) ON DELETE CASCADE;


--
-- Name: customer_profiles customer_profiles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_profiles
    ADD CONSTRAINT customer_profiles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: notifications notifications_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: payments payments_booking_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payments
    ADD CONSTRAINT payments_booking_id_fkey FOREIGN KEY (booking_id) REFERENCES public.bookings(booking_id) ON DELETE RESTRICT;


--
-- Name: reviews reviews_booking_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT reviews_booking_id_fkey FOREIGN KEY (booking_id) REFERENCES public.bookings(booking_id) ON DELETE CASCADE;


--
-- Name: reviews reviews_customer_profile_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT reviews_customer_profile_id_fkey FOREIGN KEY (customer_profile_id) REFERENCES public.customer_profiles(customer_profile_id) ON DELETE SET NULL;


--
-- Name: reviews reviews_worker_profile_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT reviews_worker_profile_id_fkey FOREIGN KEY (worker_profile_id) REFERENCES public.worker_profiles(worker_profile_id) ON DELETE SET NULL;


--
-- Name: service_requests service_requests_category_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.service_requests
    ADD CONSTRAINT service_requests_category_id_fkey FOREIGN KEY (category_id) REFERENCES public.service_categories(category_id) ON DELETE RESTRICT;


--
-- Name: service_requests service_requests_customer_profile_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.service_requests
    ADD CONSTRAINT service_requests_customer_profile_id_fkey FOREIGN KEY (customer_profile_id) REFERENCES public.customer_profiles(customer_profile_id) ON DELETE RESTRICT;


--
-- Name: worker_profiles worker_profiles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.worker_profiles
    ADD CONSTRAINT worker_profiles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: worker_skills worker_skills_category_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.worker_skills
    ADD CONSTRAINT worker_skills_category_id_fkey FOREIGN KEY (category_id) REFERENCES public.service_categories(category_id) ON DELETE CASCADE;


--
-- Name: worker_skills worker_skills_worker_profile_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.worker_skills
    ADD CONSTRAINT worker_skills_worker_profile_id_fkey FOREIGN KEY (worker_profile_id) REFERENCES public.worker_profiles(worker_profile_id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict WdvlGkvHEpfPVDufYm5aBDf2S14C9N3OB3izhNS5cgHDQaM519IWEdVoKO4zd8f

