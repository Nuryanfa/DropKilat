# Design — DropKilat

UI language: **Bahasa Indonesia**, casual-polite ("kamu", not "Anda"). Docs and code: English.

## 1. Principles

1. **Phone first, slow network first.** The buyer page must work on a cheap Android phone on 3G. Target first render < 1.5 s, page weight < 150 KB excluding product photos.
2. **One decision per screen.** During a drop the buyer only ever sees one primary button.
3. **The timer is the product.** The hold countdown is always visible while a hold is active.
4. **Trust before payment.** Seller verification and track record appear above the fold, before the Ambil button.
5. **Never blame the buyer.** Sold out, expired and rejected states explain what happens next.

## 2. Visual tokens

| Token | Value | Use |
| --- | --- | --- |
| `--ink` | `#1B1530` | Text, dark surfaces |
| `--cream` | `#FAF6EF` | Page background |
| `--lavender` | `#EFE9F7` | Secondary surface, cards |
| `--coral` | `#FF5A36` | Primary action, active hold (text on it: `--ink`) |
| `--coral-text` | `#C23A17` | Coral used as text on light backgrounds (contrast ≥ 4.5:1) |
| `--amber` | `#FFC83D` | Highlights, waitlist badge (text on it: `--ink`) |
| `--muted` | `#4A4560` | Secondary text |
| `--success` | `#1F7A4D` | Paid, verified |
| `--danger` | `#B3261E` | Rejected, report |

Typography: **Space Grotesk** (headings, numbers, timer) and **DM Sans** (body). Scale: 32 / 24 / 18 / 16 / 14 px on mobile. Tabular numbers for the timer and stock count.

Radius 12 px for cards, 999 px for pills. Spacing scale 4 / 8 / 12 / 16 / 24 / 32. Minimum touch target 44 × 44 px.

## 3. Buyer screens (mobile web)

### B1 Drop page — before opening
- Shop header: name, avatar, badges (Nomor terverifikasi, Identitas terverifikasi, @handle), track record line ("12 drop · 340 pesanan · 97% diterima") or "Seller baru".
- Product photos (swipe), title, variants with price.
- Countdown "Drop dibuka dalam 00:12:30".
- Button **Verifikasi WhatsApp dulu** (if not verified) so the buyer is ready before opening.
- Footer: "Bayar hanya lewat halaman ini. Jangan transfer ke rekening pribadi di luar instruksi halaman ini."

### B2 Drop page — live
- Remaining stock per variant, updated live: "Sisa 23 dari 50".
- Variant picker (chips), quantity (if max per buyer > 1).
- Primary button **Ambil**. While the request is in flight: "Mengamankan…" (button disabled to prevent double taps).

### B3 Hold active
- Large timer "14:59" (coral), "Stok dikunci untukmu".
- Manual mode: seller's payment destination (QRIS image and/or bank account with copy button), exact amount, upload proof button.
- Otomatis mode: dynamic QRIS, "Status diperbarui otomatis".
- Secondary text: "Link bayar juga dikirim ke WhatsApp kamu."
- At 2:00 left the timer turns amber with "Segera selesaikan pembayaran".

### B4 Waiting for seller confirmation (Manual)
- "Bukti terkirim. Menunggu konfirmasi seller (biasanya < 1 jam)." Stock stays locked.

### B5 Paid
- "Lunas" check, order code, address form (name, phone, full address, postal code, notes).

### B6 Waitlist
- "Stok sedang habis. Kamu di antrean nomor 3." Explanation: "Kalau ada yang tidak bayar, kami kirim WhatsApp dan kamu dapat 15 menit untuk bayar."
- Option "Keluar dari antrean".

### B7 Expired / Rejected / Sold out
- Expired: "Waktu bayar habis, stok kembali ke antrean." Button "Masuk antrean" if the drop is still live.
- Rejected: reason from seller + "Uang kamu akan dikembalikan oleh seller" (Manual) or "Refund otomatis diproses" (Otomatis).
- Sold out with empty waitlist closed: "Drop sudah selesai. Ikuti @handle untuk drop berikutnya."

### B8 Order status
- Timeline: Lunas → Dikemas → Dikirim (resi + courier) → Diterima.
- Buttons: "Barang sudah diterima", "Laporkan masalah".

## 4. Seller screens (responsive web, laptop and phone)

| ID | Screen | Key elements |
| --- | --- | --- |
| S1 | Login | WA number → OTP |
| S2 | Onboarding checklist | Verify WA ✓, identity, payment destination (Manual) or sub-account (Otomatis), link IG/TikTok |
| S3 | Drops list | Status pills (Draft, Terjadwal, Live, Selesai), quick stats |
| S4 | Drop editor | Photos, title, description, variants table (name, price, stock), open/close time, hold minutes, max per buyer, payment mode, preview |
| S5 | Live monitor | Big counters: Tersedia / Dikunci / Lunas / Antrean; feed of events; **Tutup drop** emergency button (confirm dialog) |
| S6 | Orders | Filters by status; Manual proofs queue with image viewer and Confirm / Reject; bulk tracking number upload; CSV export; label print view |
| S7 | Shop profile | Name, slug, avatar, payment destination, linked handles, badges status |
| S8 | Reports | Buyer reports with status |

## 5. Components

`TrustHeader`, `Countdown`, `StockMeter`, `VariantChips`, `PrimaryButton` (with in-flight state), `HoldTimer`, `PaymentInstructions` (manual | qris), `ProofUploader`, `WaitlistCard`, `OrderTimeline`, `StatusPill`, `ConfirmDialog`, `EventFeed`, `EmptyState`.

## 6. Copy rules

- Buttons are verbs: "Ambil", "Bayar", "Kirim bukti", "Konfirmasi", "Tolak".
- Times in the seller's time zone with label (WIB/WITA/WIT).
- Money: "Rp150.000" (dot thousands, no decimals).
- Never use "gagal" alone; say what failed and what to do.

## 7. Accessibility

- Contrast ≥ 4.5:1 for text; coral never used for body text on cream (use `--coral-text`).
- Timer announced to screen readers every minute (`aria-live="polite"`), not every second.
- Status is never conveyed by color alone; every pill has a word.

## 8. Prototype

`prototype/` holds a clickable HTML prototype (no backend) for seller interviews: B1 → B2 → B3 with a "Simulasikan pembeli pertama tidak bayar" button that moves stock to waitlist user #1 and shows a mock WA message. Built in task T-001.
