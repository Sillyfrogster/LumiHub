const hour = 60 * 60 * 1000;
const day = 24 * hour;

export function remainingDeletionWindow(
  deadline: string,
  now = new Date(),
): string {
  const remaining = new Date(deadline).getTime() - now.getTime();
  if (remaining <= 0) return "Recovery window ended";
  if (remaining >= day) {
    const days = Math.ceil(remaining / day);
    return `${days} ${days === 1 ? "day" : "days"} remaining`;
  }
  const hours = Math.ceil(remaining / hour);
  return `${hours} ${hours === 1 ? "hour" : "hours"} remaining`;
}
