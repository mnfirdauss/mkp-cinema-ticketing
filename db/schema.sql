-- =====================================================================
--  Cinema Online Ticketing - PostgreSQL Schema
--  Backend Development Test - Mitra Kasih Perkasa 2025
--
--  Import:  psql -U postgres -d cinema -f db/schema.sql
--           psql -U postgres -d cinema -f db/seed.sql
-- =====================================================================

CREATE EXTENSION IF NOT EXISTS pgcrypto;   -- gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS btree_gist; -- exclusion constraint jadwal bentrok

-- ---------------------------------------------------------------------
--  ENUM TYPES
-- ---------------------------------------------------------------------
CREATE TYPE user_role          AS ENUM ('customer', 'cinema_admin', 'super_admin');
CREATE TYPE studio_type        AS ENUM ('regular', 'premiere', 'imax', '4dx');
CREATE TYPE seat_type          AS ENUM ('regular', 'vip', 'sweetbox', 'wheelchair');
CREATE TYPE showtime_status    AS ENUM ('scheduled', 'open', 'closed', 'cancelled', 'finished');
CREATE TYPE seat_status        AS ENUM ('available', 'held', 'sold', 'blocked');
CREATE TYPE booking_status     AS ENUM ('pending', 'paid', 'expired', 'cancelled', 'refunded');
CREATE TYPE payment_status     AS ENUM ('pending', 'success', 'failed', 'expired', 'refunded');
CREATE TYPE refund_status      AS ENUM ('requested', 'processing', 'success', 'failed');
CREATE TYPE refund_initiator   AS ENUM ('cinema', 'customer', 'system');
CREATE TYPE ticket_status      AS ENUM ('active', 'used', 'void');

-- ---------------------------------------------------------------------
--  MASTER DATA: lokasi & bioskop
-- ---------------------------------------------------------------------
CREATE TABLE cities (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    province    VARCHAR(100) NOT NULL,
    timezone    VARCHAR(50)  NOT NULL DEFAULT 'Asia/Jakarta', -- WIB / WITA / WIT
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (name, province)
);

CREATE TABLE cinemas (
    id          BIGSERIAL PRIMARY KEY,
    city_id     BIGINT       NOT NULL REFERENCES cities(id),
    code        VARCHAR(20)  NOT NULL UNIQUE,
    name        VARCHAR(150) NOT NULL,
    address     TEXT         NOT NULL,
    phone       VARCHAR(30),
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_cinemas_city ON cinemas(city_id);

CREATE TABLE studios (
    id          BIGSERIAL PRIMARY KEY,
    cinema_id   BIGINT       NOT NULL REFERENCES cinemas(id),
    name        VARCHAR(50)  NOT NULL,
    type        studio_type  NOT NULL DEFAULT 'regular',
    total_seats INT          NOT NULL DEFAULT 0,
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (cinema_id, name)
);

-- Layout fisik kursi per studio (template, tidak berubah per jadwal)
CREATE TABLE seats (
    id          BIGSERIAL PRIMARY KEY,
    studio_id   BIGINT      NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    row_label   VARCHAR(3)  NOT NULL,   -- A, B, C ...
    seat_number INT         NOT NULL,   -- 1, 2, 3 ...
    type        seat_type   NOT NULL DEFAULT 'regular',
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE, -- kursi rusak = false
    UNIQUE (studio_id, row_label, seat_number)
);

-- ---------------------------------------------------------------------
--  USERS
-- ---------------------------------------------------------------------
CREATE TABLE users (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name     VARCHAR(150) NOT NULL,
    email         VARCHAR(150) NOT NULL UNIQUE,
    phone         VARCHAR(30),
    password_hash VARCHAR(255) NOT NULL,          -- bcrypt
    role          user_role    NOT NULL DEFAULT 'customer',
    cinema_id     BIGINT       REFERENCES cinemas(id), -- untuk cinema_admin
    is_active     BOOLEAN      NOT NULL DEFAULT TRUE,
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------
--  FILM & JADWAL TAYANG
-- ---------------------------------------------------------------------
CREATE TABLE movies (
    id               BIGSERIAL PRIMARY KEY,
    title            VARCHAR(200) NOT NULL,
    synopsis         TEXT,
    duration_minutes INT          NOT NULL CHECK (duration_minutes > 0),
    rating           VARCHAR(10)  NOT NULL DEFAULT 'SU',  -- SU, 13+, 17+, 21+
    genre            VARCHAR(100),
    poster_url       TEXT,
    release_date     DATE,
    is_active        BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE showtimes (
    id          BIGSERIAL PRIMARY KEY,
    movie_id    BIGINT          NOT NULL REFERENCES movies(id),
    studio_id   BIGINT          NOT NULL REFERENCES studios(id),
    start_time  TIMESTAMPTZ     NOT NULL,
    end_time    TIMESTAMPTZ     NOT NULL,
    price       NUMERIC(12,2)   NOT NULL CHECK (price >= 0),
    status      showtime_status NOT NULL DEFAULT 'scheduled',
    cancel_reason TEXT,
    created_by  UUID            REFERENCES users(id),
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ     NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    CHECK (end_time > start_time),
    -- Satu studio tidak boleh punya 2 jadwal aktif yang waktunya overlap
    CONSTRAINT no_overlapping_showtime EXCLUDE USING gist (
        studio_id WITH =,
        tstzrange(start_time, end_time, '[)') WITH &&
    ) WHERE (deleted_at IS NULL AND status <> 'cancelled')
);
CREATE INDEX idx_showtimes_movie ON showtimes(movie_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_showtimes_studio_start ON showtimes(studio_id, start_time) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------
--  INVENTORY KURSI PER JADWAL  (inti anti double-booking)
--  1 baris = 1 kursi pada 1 jadwal. Dibuat otomatis saat jadwal dibuat.
-- ---------------------------------------------------------------------
CREATE TABLE showtime_seats (
    id          BIGSERIAL PRIMARY KEY,
    showtime_id BIGINT      NOT NULL REFERENCES showtimes(id) ON DELETE CASCADE,
    seat_id     BIGINT      NOT NULL REFERENCES seats(id),
    status      seat_status NOT NULL DEFAULT 'available',
    booking_id  UUID,                         -- FK ditambah setelah bookings dibuat
    held_until  TIMESTAMPTZ,                  -- batas waktu hold (mis. 10 menit)
    version     INT         NOT NULL DEFAULT 0, -- optimistic locking
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (showtime_id, seat_id)
);
CREATE INDEX idx_showtime_seats_status ON showtime_seats(showtime_id, status);
CREATE INDEX idx_showtime_seats_held ON showtime_seats(held_until) WHERE status = 'held';

-- ---------------------------------------------------------------------
--  TRANSAKSI
-- ---------------------------------------------------------------------
CREATE TABLE bookings (
    id             UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_code   VARCHAR(20)    NOT NULL UNIQUE,
    user_id        UUID           NOT NULL REFERENCES users(id),
    showtime_id    BIGINT         NOT NULL REFERENCES showtimes(id),
    seat_count     INT            NOT NULL CHECK (seat_count > 0),
    total_amount   NUMERIC(12,2)  NOT NULL,
    status         booking_status NOT NULL DEFAULT 'pending',
    expires_at     TIMESTAMPTZ    NOT NULL,   -- = held_until kursi
    idempotency_key VARCHAR(100)  UNIQUE,     -- cegah double submit
    paid_at        TIMESTAMPTZ,
    cancelled_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ    NOT NULL DEFAULT now()
);
CREATE INDEX idx_bookings_user ON bookings(user_id, created_at DESC);
CREATE INDEX idx_bookings_showtime ON bookings(showtime_id, status);
CREATE INDEX idx_bookings_pending_exp ON bookings(expires_at) WHERE status = 'pending';

ALTER TABLE showtime_seats
    ADD CONSTRAINT fk_showtime_seats_booking FOREIGN KEY (booking_id) REFERENCES bookings(id);

CREATE TABLE booking_items (
    id               BIGSERIAL PRIMARY KEY,
    booking_id       UUID          NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    showtime_seat_id BIGINT        NOT NULL REFERENCES showtime_seats(id),
    price            NUMERIC(12,2) NOT NULL,
    UNIQUE (booking_id, showtime_seat_id)
);

CREATE TABLE payments (
    id               UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_id       UUID           NOT NULL REFERENCES bookings(id),
    provider         VARCHAR(50)    NOT NULL,  -- midtrans, xendit, dll
    method           VARCHAR(50)    NOT NULL,  -- va_bca, qris, gopay ...
    provider_ref     VARCHAR(100)   UNIQUE,    -- id transaksi di payment gateway
    amount           NUMERIC(12,2)  NOT NULL,
    status           payment_status NOT NULL DEFAULT 'pending',
    raw_callback     JSONB,
    paid_at          TIMESTAMPTZ,
    created_at       TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ    NOT NULL DEFAULT now()
);
CREATE INDEX idx_payments_booking ON payments(booking_id);

-- E-ticket (1 tiket per kursi) - QR code dipindai di pintu studio
CREATE TABLE tickets (
    id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_item_id  BIGINT        NOT NULL UNIQUE REFERENCES booking_items(id),
    ticket_code      VARCHAR(40)   NOT NULL UNIQUE,  -- isi QR code
    status           ticket_status NOT NULL DEFAULT 'active',
    used_at          TIMESTAMPTZ,
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------
--  REFUND / PEMBATALAN
-- ---------------------------------------------------------------------
CREATE TABLE refunds (
    id            UUID             PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_id    UUID             NOT NULL REFERENCES bookings(id),
    payment_id    UUID             NOT NULL REFERENCES payments(id),
    initiated_by  refund_initiator NOT NULL,
    reason        TEXT             NOT NULL,
    amount        NUMERIC(12,2)    NOT NULL,
    status        refund_status    NOT NULL DEFAULT 'requested',
    provider_ref  VARCHAR(100),
    processed_by  UUID             REFERENCES users(id),
    processed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ      NOT NULL DEFAULT now(),
    UNIQUE (booking_id)  -- 1 booking hanya bisa di-refund sekali
);
CREATE INDEX idx_refunds_status ON refunds(status);

-- ---------------------------------------------------------------------
--  PENCATATAN / AUDIT (restok & histori kursi)
-- ---------------------------------------------------------------------
CREATE TABLE seat_status_logs (
    id               BIGSERIAL PRIMARY KEY,
    showtime_seat_id BIGINT      NOT NULL REFERENCES showtime_seats(id) ON DELETE CASCADE,
    from_status      seat_status,
    to_status        seat_status NOT NULL,
    booking_id       UUID,
    reason           VARCHAR(50) NOT NULL, -- hold, paid, hold_expired, refund, restock, block
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_seat_logs_seat ON seat_status_logs(showtime_seat_id, created_at);

-- Trigger: setiap perubahan status kursi otomatis tercatat (untuk rekonsiliasi restok)
CREATE OR REPLACE FUNCTION log_seat_status_change() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'UPDATE' AND NEW.status IS DISTINCT FROM OLD.status THEN
        INSERT INTO seat_status_logs(showtime_seat_id, from_status, to_status, booking_id, reason)
        VALUES (NEW.id, OLD.status, NEW.status, COALESCE(NEW.booking_id, OLD.booking_id),
                COALESCE(current_setting('app.seat_reason', true), 'update'));
    END IF;
    NEW.updated_at := now();
    NEW.version    := OLD.version + 1;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_showtime_seats_log
BEFORE UPDATE ON showtime_seats
FOR EACH ROW EXECUTE FUNCTION log_seat_status_change();

-- View ringkasan penjualan per jadwal (untuk laporan / dashboard)
CREATE VIEW v_showtime_availability AS
SELECT s.id AS showtime_id,
       COUNT(*)                                      AS total_seats,
       COUNT(*) FILTER (WHERE ss.status = 'available') AS available,
       COUNT(*) FILTER (WHERE ss.status = 'held')      AS held,
       COUNT(*) FILTER (WHERE ss.status = 'sold')      AS sold,
       COUNT(*) FILTER (WHERE ss.status = 'blocked')   AS blocked
FROM showtimes s
JOIN showtime_seats ss ON ss.showtime_id = s.id
WHERE s.deleted_at IS NULL
GROUP BY s.id;
