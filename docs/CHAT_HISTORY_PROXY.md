# Chat History Proxy

Layanan Go mandiri, **tanpa database**, yang jadi proxy read-only antara app
mobile/web klien dan REST API admin Qiscus. Ditulis generik — tidak terikat
klien tertentu — supaya bisa dipakai ulang untuk klien mana pun yang butuh
menampilkan riwayat percakapan lintas sesi di app *sessional* mode mereka.

## Kapan layanan ini relevan

App yang jalan dalam mode Qiscus *sessional* mengganti room chat aktifnya
setiap kali sesi selesai (resolved). Widget/SDK sisi app cuma menyimpan
referensi ke **satu** room aktif — begitu sesi lama resolved, app tidak lagi
punya cara membaca isi room lama itu lewat token user biasa (token itu sudah
tidak valid untuk room yang bukan room aktifnya).

Layanan ini menjembatani itu: dia menukar identitas (JWT milik user →
kredensial **server** Qiscus) supaya bisa membaca riwayat sesi-sesi lama milik
user tersebut, tanpa app perlu menyimpan salinan chat sendiri.

## Arsitektur

- **Tanpa database, tanpa webhook.** Riwayat diambil langsung dari Qiscus
  saat diminta (`GET /api/v1/sessions`, `GET
  /api/v1/sessions/{room_id}/messages`), bukan disalin ke storage sendiri.
- Cache in-memory pendek (default 60 detik) per user, untuk membatasi beban ke
  Qiscus. Bisa dilewati per-request dengan `?fresh=1` (dipakai app tepat
  setelah membuat room baru, supaya tidak menunggu TTL cache).
- Autentikasi: JWT RS256, public key dari environment. Token **diterbitkan
  oleh backend auth klien sendiri** — layanan ini cuma memvalidasinya, tidak
  pernah menerbitkan token.
- Verifikasi kepemilikan room dilakukan sebelum mengirim transkrip — `sub`
  dari JWT harus punya room itu di daftar sesinya, kalau tidak dibalas `404`
  (bukan `403`, supaya tidak membocorkan keberadaan room orang lain).

## Konfigurasi (semua lewat environment variable)

| Env | Wajib | Default | Keterangan |
|---|---|---|---|
| `APP_PORT` | tidak | `8081` | Port HTTP |
| `QISCUS_APP_ID` | **ya** | — | App ID Qiscus klien |
| `QISCUS_SECRET_KEY` | **ya** | — | Secret server Qiscus, tidak pernah dikirim ke klien app |
| `QISCUS_BASE_URL` | tidak | `https://api3.qiscus.com` | |
| `JWT_PUBLIC_KEY` | **ya** | — | Public key RSA (PEM) untuk memverifikasi JWT dari auth klien |
| `CACHE_TTL_SECONDS` | tidak | `60` | Umur cache daftar sesi per user |

Tidak ada nilai default untuk kredensial (`QISCUS_APP_ID`,
`QISCUS_SECRET_KEY`, `JWT_PUBLIC_KEY`) — kalau salah satu kosong, service
**gagal start** dengan pesan jelas menyebut env mana yang kosong, bukan diam
diam jalan dengan kredensial kosong. Ini yang membuat pindah dari satu klien
ke klien lain semudah mengganti isi `.env` — tidak ada satu baris kode pun
yang perlu diubah.

Mulai dari template:

```bash
cp .env.example .env
# isi QISCUS_APP_ID, QISCUS_SECRET_KEY, QISCUS_BASE_URL (kalau bukan default),
# dan JWT_PUBLIC_KEY (public key milik backend auth klien, BUKAN dibuat sendiri
# oleh layanan ini)

set -a; source .env; set +a
go run ./cmd/chat-history-proxy
```

## Kontrak API

```
GET /api/v1/sessions
  Header: Authorization: Bearer <JWT RS256, klaim `sub` = user_id klien>
  -> { "data": [ { "room_id", "name", "started_at", "is_resolved", "last_message", "topic" } ], "next_cursor": null }

GET /api/v1/sessions/{room_id}/messages
  -> { "data": [ { "id", "sender_role", "sender_name", "type", "text", "payload", "created_at" } ], "next_cursor": null }
```

`topic` kosong kalau app pemanggil tidak menyimpannya secara eksplisit lewat
`qiscus.updateChatRoom(..., extras)` — lihat repo widget/app untuk pola pre-chat
topic dialog yang dipakai contoh implementasinya.

## Batasan yang diketahui (jujur, bukan disembunyikan)

- **`get_user_rooms` Qiscus tidak menyediakan `last_comment`/`room_created_at`.**
  `started_at` dan `last_message` diisi lewat **satu panggilan `load_comments`
  tambahan per room** (N+1 untuk N room) — biaya nyata, diterima untuk skala
  jumlah room per user yang wajar (bukan ribuan).
- **`load_comments` Qiscus mengembalikan pesan NEWEST FIRST (descending)**,
  bukan oldest-first — kalau menyentuh lagi `internal/proxy/qiscus/client.go`,
  jangan asumsikan urutan tanpa `curl` langsung ke data yang representatif
  (banyak pesan, span waktu jauh). Ini pernah salah diverifikasi sebelumnya
  dengan fixture 2 pesan yang kebetulan ascending.
- Verifikasi terhadap Qiscus asli sejauh ini pakai app Qiscus milik tim
  internal sendiri, bukan app produksi klien — bentuk respons Qiscus
  kemungkinan besar berlaku umum, tapi cek ulang kalau ada anomali di klien
  baru.

## Struktur kode

```
cmd/chat-history-proxy/main.go   # entry point, wiring
cmd/devtools/gen-dev-jwt/        # LOCAL DEV ONLY — generator JWT buat testing tanpa auth klien asli
internal/proxy/
  config/     # loader environment, gagal start kalau kredensial wajib kosong
  qiscus/     # klien REST admin Qiscus (get_user_rooms, load_comments)
  cache/      # TTL cache in-memory generik, tanpa dependency eksternal
  middleware/ # verifikasi JWT RS256
  handler/    # /api/v1/sessions, /api/v1/sessions/{room_id}/messages
internal/middleware/logger.go  # middleware generik, dipakai ulang dari service lain kalau ada di repo yang sama
```

## Test

```bash
go build ./...
go vet ./...
go test ./...
```
