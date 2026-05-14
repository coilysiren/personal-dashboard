// Voice playback helper. Panels call window.dashboardSay(text) to play
// an event announcement. Browsers block audio without a user gesture,
// so the first call must come from a button/tap; subsequent calls in
// the same tab inherit the unlocked state.
//
// API:
//   window.dashboardSay(text)         - play, returns Promise.
//   window.dashboardVoiceEnabled()    - cheap probe (true if the daemon
//                                       reports voice configured).
window.dashboardSay = async function (text) {
  if (!text) return;
  try {
    const res = await fetch("/api/voice/say", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: "text=" + encodeURIComponent(text),
    });
    if (!res.ok) {
      console.warn("voice say failed:", res.status, await res.text());
      return;
    }
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const audio = new Audio(url);
    audio.addEventListener("ended", () => URL.revokeObjectURL(url));
    await audio.play();
  } catch (err) {
    console.warn("voice say error:", err);
  }
};
