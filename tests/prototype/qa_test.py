"""QA verification suite for T-001 / T-002: accessibility, copy, 360px viewport, contrast, and state invariants."""
from pathlib import Path
from playwright.sync_api import sync_playwright, expect

ROOT = Path(__file__).resolve().parents[2]
PROTO = ROOT / "prototype/index.html"


def run():
    with sync_playwright() as p:
        browser = p.chromium.launch(channel="msedge", headless=True)
        page = browser.new_page(viewport={"width": 360, "height": 800})
        page.clock.install()
        page.goto(PROTO.as_uri())

        def assert_no_overflow(phase_name: str):
            overflow = page.evaluate("document.documentElement.scrollWidth > window.innerWidth")
            assert not overflow, f"Horizontal overflow in {phase_name}"

        def assert_button_tap_targets(phase_name: str):
            bad = page.evaluate("""() => {
                const items = [];
                for (const el of document.querySelectorAll('button')) {
                    const style = getComputedStyle(el);
                    if (style.display === 'none' || style.visibility === 'hidden' || el.closest('[hidden]')) continue;
                    const r = el.getBoundingClientRect();
                    if (r.width < 44 || r.height < 44) {
                        items.push({ id: el.id || el.className, w: r.width, h: r.height });
                    }
                }
                return items;
            }""")
            assert not bad, f"Buttons below 44x44px in {phase_name}: {bad}"

        def assert_contrast(phase_name: str):
            bad = page.evaluate("""() => {
                function parseColor(c) {
                    const m = c.match(/rgba?\\((\\d+),\\s*(\\d+),\\s*(\\d+)(?:,\\s*([\\d.]+))?\\)/);
                    if (!m) return null;
                    return [ +m[1], +m[2], +m[3], m[4] === undefined ? 1 : +m[4] ];
                }
                function blend(fg, bg) {
                    const a = fg[3];
                    return [
                        Math.round(fg[0] * a + bg[0] * (1 - a)),
                        Math.round(fg[1] * a + bg[1] * (1 - a)),
                        Math.round(fg[2] * a + bg[2] * (1 - a)),
                        1
                    ];
                }
                function getBg(el) {
                    const stack = [];
                    let cur = el;
                    while (cur && cur.nodeType === 1) {
                        const c = parseColor(getComputedStyle(cur).backgroundColor);
                        if (c && c[3] > 0) stack.unshift(c);
                        cur = cur.parentElement;
                    }
                    let bg = [250, 246, 239, 1];
                    for (const c of stack) bg = blend(c, bg);
                    return bg;
                }
                function lum(rgb) {
                    const a = rgb.slice(0, 3).map(v => {
                        v /= 255;
                        return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
                    });
                    return a[0] * 0.2126 + a[1] * 0.7152 + a[2] * 0.0722;
                }
                function ratio(c1, c2) {
                    const l1 = lum(c1), l2 = lum(c2);
                    return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
                }
                const results = [];
                const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
                let node;
                while ((node = walker.nextNode())) {
                    if (!node.nodeValue.trim()) continue;
                    const el = node.parentElement;
                    if (!el || el.closest('[hidden]') || el.classList.contains('sr-only') || el.classList.contains('skip')) continue;
                    if (el.closest('button:disabled')) continue;
                    const st = getComputedStyle(el);
                    if (st.display === 'none' || st.visibility === 'hidden') continue;
                    const bg = getBg(el);
                    const fg = blend(parseColor(st.color), bg);
                    const r = ratio(fg, bg);
                    if (r < 4.5) {
                        results.push({ text: node.nodeValue.trim(), ratio: Math.round(r * 100) / 100 });
                    }
                }
                return results;
            }""")
            assert not bad, f"Contrast < 4.5:1 in {phase_name}: {bad}"

        # 1. Skip link keyboard focus and activation
        page.keyboard.press("Tab")
        expect(page.locator(".skip")).to_be_focused()
        assert page.locator(".skip").bounding_box()["height"] >= 44
        page.keyboard.press("Enter")

        # 2. B1 Before Opening copy & checks
        expect(page.locator(".badges")).to_contain_text("Nomor terverifikasi")
        expect(page.locator(".badges")).to_contain_text("Identitas terverifikasi")
        expect(page.locator(".track")).to_have_text("12 drop · 340 pesanan · 97% diterima")
        expect(page.locator("#before h3")).to_have_text("Drop dibuka dalam 00:12:30")
        expect(page.locator("#verify")).to_have_text("Verifikasi WhatsApp dulu")
        expect(page.locator(".safety")).to_have_text(
            "Bayar hanya lewat halaman ini. Jangan transfer ke rekening pribadi di luar instruksi halaman ini."
        )
        assert_no_overflow("B1-before")
        assert_button_tap_targets("B1-before")
        assert_contrast("B1-before")

        # 3. B2 Live & in-flight state
        page.locator("#verify").click()
        page.locator("#open").click()
        expect(page.locator("#drop-status")).to_have_text("Live")
        expect(page.locator("#claim")).to_have_text("Ambil")
        assert_no_overflow("B2-live")
        assert_button_tap_targets("B2-live")
        assert_contrast("B2-live")

        page.locator("#claim").click()
        expect(page.locator("#claim")).to_be_disabled()
        expect(page.locator("#claim")).to_have_text("Mengamankan…")
        page.clock.run_for(400)

        # 4. B3 Hold active, sticky timer, QRIS placeholder, and polite per-minute announcements
        expect(page.locator("#hold")).to_be_visible()
        expect(page.locator("#active-hold")).to_be_visible()
        expect(page.locator("#active-hold")).to_contain_text("Stok dikunci untukmu")
        expect(page.locator(".qris")).to_contain_text("Contoh")
        expect(page.locator("#proof")).to_have_text("Kirim bukti")
        expect(page.locator("#hold")).to_contain_text("Link bayar juga dikirim ke WhatsApp kamu.")
        assert_no_overflow("B3-hold")
        assert_button_tap_targets("B3-hold")
        assert_contrast("B3-hold")

        # Verify screen-reader timer announces only once per minute, not every second
        sr_updates = []
        page.expose_function("onSrTimer", lambda text: sr_updates.append(text))
        page.evaluate("""() => {
            const el = document.getElementById('timer-announcement');
            new MutationObserver(() => window.onSrTimer(el.textContent)).observe(
                el, { childList: true, characterData: true, subtree: true }
            );
        }""")
        # Advance 59 seconds in 1-second steps: still in minute 15 -> 14 boundary once
        for _ in range(120):
            page.clock.run_for(1_000)
        assert sr_updates == [
            "Sisa waktu bayar 14 menit.",
            "Sisa waktu bayar 13 menit.",
        ], f"Expected 1 announcement per minute, got: {sr_updates}"

        # 5. Urgent timer at <= 2:00 remaining
        page.locator("#speed").click()
        expect(page.locator("#speed")).to_have_attribute("aria-pressed", "true")
        page.clock.run_for(22_000)
        expect(page.locator("#urgency")).to_be_visible()
        expect(page.locator("#urgency")).to_have_text("Segera selesaikan pembayaran")
        assert_no_overflow("B3-urgent")
        assert_button_tap_targets("B3-urgent")
        assert_contrast("B3-urgent")

        # 6. B7 Expired
        page.clock.run_for(5_000)
        expect(page.locator("#expired")).to_be_visible()
        expect(page.locator("#expired h3")).to_have_text("Waktu bayar habis, stok kembali ke antrean.")
        expect(page.locator("#rejoin")).to_have_text("Masuk antrean")
        assert_no_overflow("B7-expired")
        assert_button_tap_targets("B7-expired")
        assert_contrast("B7-expired")

        # 7. B4 Proof submitted (never expires even under 30x speed)
        page.locator("#rejoin").click()
        page.locator("#proof").click()
        expect(page.locator("#confirmation h3")).to_have_text(
            "Bukti terkirim. Menunggu konfirmasi seller (biasanya < 1 jam)."
        )
        page.clock.run_for(60_000)
        expect(page.locator("#confirmation")).to_be_visible()
        expect(page.locator("#stock-label")).to_have_text("Sisa 9 dari 10")
        assert_no_overflow("B4-confirmation")
        assert_button_tap_targets("B4-confirmation")
        assert_contrast("B4-confirmation")

        # 8. B6 Waitlist & FIFO promotion
        page.reload()
        page.locator("#rush").click()
        page.clock.run_for(2_500)
        expect(page.locator("#waiting")).to_be_visible()
        expect(page.locator("#queue-copy")).to_have_text("Stok sedang habis. Kamu di antrean nomor 3.")
        expect(page.locator("#waiting")).to_contain_text(
            "Kalau ada yang tidak bayar, kami kirim WhatsApp dan kamu dapat 15 menit untuk bayar."
        )
        expect(page.locator("#leave")).to_have_text("Keluar dari antrean")
        page.locator("#expire").click()
        expect(page.locator("#whatsapp")).to_be_visible()
        expect(page.locator("#whatsapp")).to_contain_text("Giliranmu! Bayar dalam 15 menit")
        assert_no_overflow("B6-waiting")
        assert_button_tap_targets("B6-waiting")
        assert_contrast("B6-waiting")

        browser.close()
    print("PASS: QA accessibility, contrast, tap-target, copy, and timer announcement suite")


if __name__ == "__main__":
    run()
