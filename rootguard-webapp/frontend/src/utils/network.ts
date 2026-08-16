// The browser already reached the current page over the host's LAN IP
// (RootGuard's documented access pattern is http://<host-LAN-IP>:8080, not
// localhost) - window.location.hostname is that address, so it's a better
// setup default than 0.0.0.0. Falls back to 0.0.0.0 for anything not
// directly usable as a DNS bind address: a hostname instead of an IP, an
// IPv6 literal (Core only accepts IPv4), or loopback (unreachable for other
// devices on the network).
export function detectDefaultBindAddress(hostname: string): string {
  const octets = hostname.split(".");
  const isIPv4 =
    octets.length === 4 && octets.every((octet) => /^\d{1,3}$/.test(octet) && Number(octet) <= 255);
  return isIPv4 && !hostname.startsWith("127.") ? hostname : "0.0.0.0";
}
