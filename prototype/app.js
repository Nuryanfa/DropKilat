"use strict";
(() => {
  const $ = (id) => document.getElementById(id);
  const HOLD = 15 * 60;
  const state = { phase: "before", live: false, verified: false, stock: 10,
    queue: [], holds: [], viewer: "kamu", countdown: 750, speed: 1, busy: false };
  let previous = performance.now();
  let announcedMinute = null;
  let nextBuyer = 1;

  const time = (seconds) => {
    const whole = Math.max(0, Math.ceil(seconds));
    return [Math.floor(whole / 60), whole % 60].map((n) => String(n).padStart(2, "0")).join(":");
  };
  const viewerHold = () => state.holds.find((hold) => hold.buyer === state.viewer);
  const say = (message) => { $("feedback").textContent = message; };
  const event = (message) => { $("demo-event").textContent = message; };

  function render() {
    for (const phase of ["before", "live", "hold", "waiting", "expired", "confirmation"]) {
      $(phase).hidden = state.phase !== phase;
    }
    $("stock-label").textContent = `Sisa ${state.stock} dari 10`;
    $("stock-meter").value = state.stock;
    $("drop-status").textContent = state.live ? "Live" : "Terjadwal";
    $("countdown").textContent = `00:${time(state.countdown)}`;
    $("verify").disabled = state.verified;
    $("verify").textContent = state.verified ? "Nomor terverifikasi" : "Verifikasi WhatsApp dulu";
    $("claim").disabled = state.busy;
    $("claim").textContent = state.busy ? "Mengamankan…" : (state.verified ? "Ambil" : "Verifikasi WhatsApp dulu");
    const position = state.queue.indexOf(state.viewer) + 1;
    $("queue-number").textContent = `#${position}`;
    $("queue-copy").textContent = `Stok sedang habis. Kamu di antrean nomor ${position}.`;
    const hold = viewerHold();
    $("active-hold").hidden = state.phase !== "hold";
    if (hold) $("sticky-timer").textContent = time(hold.remaining);
    if (hold && !hold.proof) {
      $("hold-timer").textContent = time(hold.remaining);
      $("hold-timer").classList.toggle("urgent", hold.remaining <= 120);
      $("urgency").hidden = hold.remaining > 120;
      const minute = Math.ceil(hold.remaining / 60);
      if (minute !== announcedMinute) {
        $("timer-announcement").textContent = `Sisa waktu bayar ${minute} menit.`;
        announcedMinute = minute;
      }
    }
    $("open").disabled = state.live;
    $("rush").disabled = state.busy || state.stock === 0;
    $("expire").disabled = state.busy || !state.holds.some((h) => !h.proof);
  }

  function openDrop() {
    state.live = true;
    state.countdown = 0;
    if (state.phase === "before") state.phase = "live";
    render();
  }

  function reserve(buyer) {
    state.stock -= 1;
    state.holds.push({ buyer, remaining: HOLD, proof: false });
    if (buyer === state.viewer) {
      state.phase = "hold";
      announcedMinute = null;
      say("Stok dikunci untukmu");
    }
  }

  function promote() {
    while (state.stock > 0 && state.queue.length > 0) {
      const buyer = state.queue.shift();
      reserve(buyer);
      $("whatsapp").hidden = false;
      $("wa-recipient").textContent = buyer === state.viewer
        ? "Pesan contoh untuk kamu. Hold 15 menit dimulai sekarang."
        : `Pesan contoh untuk ${buyer}, antrean nomor 1. Bukan pesan untuk kamu.`;
      event("Satu stok dilepas lalu langsung dikunci untuk antrean nomor 1.");
    }
  }

  function expire(hold) {
    state.holds.splice(state.holds.indexOf(hold), 1);
    state.stock += 1;
    if (hold.buyer === state.viewer) {
      state.phase = "expired";
      say("Waktu bayar habis, stok kembali ke antrean.");
    }
    promote();
  }

  function join() {
    if (state.stock > 0) reserve(state.viewer);
    else {
      if (!state.queue.includes(state.viewer)) state.queue.push(state.viewer);
      state.phase = "waiting";
      say("");
    }
    render();
  }

  $("verify").addEventListener("click", () => {
    state.verified = true;
    say("Nomor terverifikasi — simulasi, tanpa WhatsApp sungguhan.");
    render();
  });
  $("open").addEventListener("click", openDrop);
  $("claim").addEventListener("click", () => {
    if (!state.live || state.busy) return;
    if (!state.verified) {
      state.verified = true;
      say("Nomor terverifikasi — simulasi. Sekarang kamu bisa Ambil.");
      render();
      return;
    }
    state.busy = true;
    render();
    setTimeout(() => {
      join();
      state.busy = false;
      render();
    }, 350);
  });
  $("rush").addEventListener("click", () => {
    openDrop();
    state.busy = true;
    render();
    const burst = setInterval(() => {
      if (state.stock > 0) {
        reserve(`Pembeli demo ${nextBuyer++}`);
        render();
        return;
      }
      clearInterval(burst);
      if (!state.queue.includes(state.viewer) && !viewerHold()) {
        state.queue.push("Pembeli antrean 1", "Pembeli antrean 2", state.viewer);
        state.phase = "waiting";
      }
      // Fill the remaining places from the 30 competing demo buyers.
      while (state.holds.length + state.queue.length < 30) state.queue.push(`Pembeli demo ${nextBuyer++}`);
      state.busy = false;
      say("");
      event("30 pembeli berebut. Stok habis; pembeli berikutnya masuk antrean.");
      render();
    }, 180);
  });
  $("expire").addEventListener("click", () => {
    const hold = state.holds.find((h) => !h.proof);
    if (!hold) return;
    expire(hold);
    if (state.queue.length === 0 && state.stock > 0) event("Hold habis. Satu stok tersedia untuk diambil lagi.");
    render();
  });
  $("leave").addEventListener("click", () => {
    state.queue = state.queue.filter((buyer) => buyer !== state.viewer);
    state.phase = "live";
    say("Kamu sudah keluar dari antrean.");
    render();
  });
  $("rejoin").addEventListener("click", () => {
    if (!state.verified) {
      state.verified = true;
      say("Nomor terverifikasi — simulasi.");
    }
    join();
  });
  $("proof").addEventListener("click", () => {
    const hold = viewerHold();
    if (!hold || hold.remaining <= 0) return;
    hold.proof = true;
    state.phase = "confirmation";
    say("Bukti contoh terkirim. Tidak ada file yang diunggah.");
    render();
  });
  $("speed").addEventListener("click", () => {
    tick();
    state.speed = state.speed === 1 ? 30 : 1;
    $("speed").setAttribute("aria-pressed", String(state.speed === 30));
    event(state.speed === 30 ? "Timer hold berjalan 30× lebih cepat." : "Timer hold berjalan normal.");
  });

  function tick() {
    const now = performance.now();
    const elapsed = (now - previous) / 1000;
    previous = now;
    if (!state.live) {
      state.countdown = Math.max(0, state.countdown - elapsed);
      if (state.countdown === 0) openDrop();
    }
    // Snapshot first: a newly promoted hold receives its full 15 minutes.
    for (const hold of [...state.holds]) {
      if (hold.proof) continue;
      hold.remaining -= elapsed * state.speed;
      if (hold.remaining <= 0) expire(hold);
    }
    render();
  }
  setInterval(tick, 200);
  render();
})();
