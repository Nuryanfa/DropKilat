"""Prototype smoke checks. Run: pip install playwright; python tests/prototype/smoke.py.
Uses installed Edge; no browser download or application build is needed.
"""
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from threading import Thread

from playwright.sync_api import sync_playwright, expect

ROOT = Path(__file__).resolve().parents[2]
ARTIFACTS = ROOT / "tests/artifacts/T-001"


def run():
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch(channel="msedge", headless=True)
        page = browser.new_page(viewport={"width": 360, "height": 800})
        page.on("pageerror", lambda error: errors.append(str(error)))
        page.on("console", lambda message: errors.append(message.text) if message.type == "error" else None)
        requests = []
        page.on("request", lambda request: requests.append(request.url))
        page.clock.install()
        page.goto((ROOT / "prototype/index.html").as_uri())
        expect(page.locator("#countdown")).to_have_text("00:12:30")
        assert page.evaluate("document.documentElement.scrollWidth <= innerWidth")
        assert len(requests) == 3 and all(url.startswith("file:") for url in requests)
        brand = page.locator(".brand").bounding_box()
        assert brand["width"] >= 44 and brand["height"] >= 44
        header_top = page.locator(".site-header").bounding_box()["y"]
        page.keyboard.press("Tab")
        expect(page.locator(".skip")).to_be_focused()
        assert page.locator(".site-header").bounding_box()["y"] == header_top
        # Real pointer down/up catches layout shifts that can swallow a click.
        page.locator("#verify").scroll_into_view_if_needed()
        verify = page.locator("#verify").bounding_box()
        page.mouse.click(verify["x"] + verify["width"] / 2, verify["y"] + verify["height"] / 2)
        expect(page.locator("#verify")).to_be_disabled()
        page.locator("#open").click()
        page.locator("#claim").click()
        expect(page.locator("#claim")).to_be_disabled()
        page.clock.run_for(400)
        expect(page.locator("#hold")).to_be_visible()
        expect(page.locator("#stock-label")).to_have_text("Sisa 9 dari 10")
        expect(page.locator("#hold-timer")).to_have_text("15:00")
        page.screenshot(path=str(ARTIFACTS / "mobile-hold.png"), full_page=True)
        page.locator("#speed").click()
        page.clock.run_for(26_000)
        expect(page.locator("#urgency")).to_be_visible()
        assert page.locator("#hold-timer").get_attribute("class") == "timer urgent"
        page.clock.run_for(5_000)
        expect(page.locator("#expired")).to_be_visible()
        expect(page.locator("#stock-label")).to_have_text("Sisa 10 dari 10")
        page.locator("#rejoin").click()
        page.locator("#proof").click()
        page.clock.run_for(40_000)
        expect(page.locator("#confirmation")).to_be_visible()
        expect(page.locator("#stock-label")).to_have_text("Sisa 9 dari 10")

        page.reload()
        page.locator("#rush").click()
        page.clock.run_for(2_200)
        expect(page.locator("#waiting")).to_be_visible()
        expect(page.locator("#queue-number")).to_have_text("#3")
        expect(page.locator("#stock-label")).to_have_text("Sisa 0 dari 10")
        page.locator("#expire").click()
        expect(page.locator("#queue-number")).to_have_text("#2")
        expect(page.locator("#wa-recipient")).to_contain_text("Pembeli antrean 1")
        expect(page.locator("#stock-label")).to_have_text("Sisa 0 dari 10")
        page.locator("#expire").click()
        expect(page.locator("#queue-number")).to_have_text("#1")
        page.locator("#expire").click()
        expect(page.locator("#hold")).to_be_visible()
        expect(page.locator("#hold-timer")).to_have_text("15:00")
        expect(page.locator("#wa-recipient")).to_contain_text("untuk kamu")
        page.locator("#proof").click()
        expect(page.locator("#confirmation")).to_be_visible()
        expect(page.locator("#whatsapp")).to_be_hidden()

        page.reload()
        page.locator("#rush").click()
        page.clock.run_for(2_200)
        for _ in range(3):
            page.locator("#expire").click()
        expect(page.locator("#whatsapp")).to_be_visible()
        page.locator("#speed").click()
        page.clock.run_for(31_000)
        expect(page.locator("#expired")).to_be_visible()
        expect(page.locator("#whatsapp")).to_be_hidden()

        page.reload()
        page.locator("#rush").click()
        page.clock.run_for(2_200)
        page.locator("#expire").click()
        expect(page.locator("#whatsapp")).to_be_visible()
        page.locator("#leave").click()
        expect(page.locator("#whatsapp")).to_be_hidden()
        expect(page.locator("#live")).to_be_visible()
        page.locator("#claim").click()  # Simulated verification after opening.
        expect(page.locator("#claim")).to_have_text("Ambil")
        page.locator("#claim").click()
        page.clock.run_for(400)
        expect(page.locator("#waiting")).to_be_visible()
        expect(page.locator("#queue-number")).to_have_text("#19")

        handler = partial(SimpleHTTPRequestHandler, directory=str(ROOT / "prototype"))
        server = ThreadingHTTPServer(("127.0.0.1", 0), handler)
        Thread(target=server.serve_forever, daemon=True).start()
        try:
            page.set_viewport_size({"width": 1280, "height": 1000})
            page.goto(f"http://127.0.0.1:{server.server_port}/")
            expect(page.locator("#before")).to_be_visible()
            assert page.evaluate("document.documentElement.scrollWidth <= innerWidth")
            page.screenshot(path=str(ARTIFACTS / "desktop-countdown.png"), full_page=True)
            page.clock.fast_forward(751_000)
            expect(page.locator("#live")).to_be_visible()
        finally:
            server.shutdown()
            server.server_close()
        browser.close()
    assert not errors, errors
    size = sum(path.stat().st_size for path in (ROOT / "prototype").iterdir())
    assert size < 200_000, size
    print(f"PASS: file + HTTP, 360px + desktop, claim, rush, FIFO, expiry, proof, leave; {size} bytes; no console errors")


if __name__ == "__main__":
    run()
