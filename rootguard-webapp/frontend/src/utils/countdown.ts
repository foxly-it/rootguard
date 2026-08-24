// Formats a millisecond duration as "M:SS" for the AdGuard protection-pause
// countdown. Rounds up to the next full second (ceil, not floor/round) so
// the display doesn't show "0:00" for up to a second before the pause has
// actually ended.
export function formatCountdown(remainingMs: number): string {
  const totalSeconds = Math.max(0, Math.ceil(remainingMs / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
}
