-- =====================================================================
--  Seed data untuk mencoba API
--  Semua user password: password123
-- =====================================================================

INSERT INTO cities (name, province, timezone) VALUES
    ('Semarang', 'Jawa Tengah', 'Asia/Jakarta'),
    ('Jakarta Barat', 'DKI Jakarta', 'Asia/Jakarta'),
    ('Denpasar', 'Bali', 'Asia/Makassar');

INSERT INTO cinemas (city_id, code, name, address, phone) VALUES
    (1, 'SMG-PRM', 'MKP Cinema Paragon Semarang', 'Jl. Pemuda No.118, Semarang', '024-111111'),
    (2, 'JKT-CTP', 'MKP Cinema Central Park',     'Jl. Letjen S. Parman Kav.28, Jakarta Barat', '021-222222'),
    (3, 'DPS-LVB', 'MKP Cinema Level 21 Bali',    'Jl. Teuku Umar No.1, Denpasar', '0361-333333');

INSERT INTO studios (cinema_id, name, type) VALUES
    (1, 'Studio 1', 'regular'),
    (1, 'Studio 2', 'premiere'),
    (2, 'Studio 1', 'imax'),
    (3, 'Studio 1', 'regular');

-- Generate layout kursi: studio regular/imax 5 baris x 10 kursi, premiere 3 baris x 6 kursi
INSERT INTO seats (studio_id, row_label, seat_number, type)
SELECT st.id, chr(64 + r), n,
       CASE WHEN st.type = 'premiere' THEN 'vip'::seat_type ELSE 'regular'::seat_type END
FROM studios st
CROSS JOIN LATERAL generate_series(1, CASE WHEN st.type = 'premiere' THEN 3 ELSE 5 END) AS r
CROSS JOIN LATERAL generate_series(1, CASE WHEN st.type = 'premiere' THEN 6 ELSE 10 END) AS n;

UPDATE studios st SET total_seats = (SELECT COUNT(*) FROM seats s WHERE s.studio_id = st.id);

INSERT INTO movies (title, synopsis, duration_minutes, rating, genre, release_date) VALUES
    ('Pengabdi Setan 3', 'Teror berlanjut di rumah susun.', 120, '17+', 'Horror', '2025-08-01'),
    ('Laskar Pelangi Reborn', 'Kisah persahabatan anak Belitung.', 110, 'SU', 'Drama', '2025-07-15'),
    ('Garuda Strike', 'Pasukan elit menyelamatkan ibu kota.', 135, '13+', 'Action', '2025-09-01');

-- password123 (bcrypt cost 10)
INSERT INTO users (full_name, email, phone, password_hash, role, cinema_id) VALUES
    ('Super Admin',  'admin@mkp.id',    '081200000001', '$2a$10$7z/6lCNSyNTPkUlG8a9LH.Udbl251OckODwuyOq60VFl.lrGn9u3q', 'super_admin', NULL),
    ('Admin Semarang','smg@mkp.id',     '081200000002', '$2a$10$7z/6lCNSyNTPkUlG8a9LH.Udbl251OckODwuyOq60VFl.lrGn9u3q', 'cinema_admin', 1),
    ('Budi Customer','budi@mail.com',   '081200000003', '$2a$10$7z/6lCNSyNTPkUlG8a9LH.Udbl251OckODwuyOq60VFl.lrGn9u3q', 'customer', NULL);
